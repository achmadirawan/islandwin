package audio

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	mp3 "github.com/hajimehoshi/go-mp3"
)

// DecodedAudio represents a decoded PCM stream with format metadata.
type DecodedAudio struct {
	Stream     io.ReadSeeker
	SampleRate int
	Channels   int
	Length     int64 // Length in bytes
}

// DecodeFile opens and decodes an audio file (MP3 or standard WAV).
func DecodeFile(path string) (*DecodedAudio, error) {
	ext := strings.ToLower(filepath.Ext(path))
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	switch ext {
	case ".mp3":
		dec, err := mp3.NewDecoder(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return normalize(dec, dec.SampleRate(), 2, dec.Length()), nil

	case ".wav":
		stream, sampleRate, channels, length, err := decodeWAV(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return normalize(stream, sampleRate, channels, length), nil

	default:
		f.Close()
		return nil, errors.New("unsupported format: " + ext)
	}
}

// decodeWAV extracts raw PCM stream from standard RIFF/WAVE files.
func decodeWAV(f *os.File) (io.ReadSeeker, int, int, int64, error) {
	var header [44]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return nil, 0, 0, 0, err
	}

	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return nil, 0, 0, 0, errors.New("invalid WAV format")
	}

	channels := int(binary.LittleEndian.Uint16(header[22:24]))
	sampleRate := int(binary.LittleEndian.Uint32(header[24:28]))
	bits := int(binary.LittleEndian.Uint16(header[34:36]))
	dataSize := int64(binary.LittleEndian.Uint32(header[40:44]))

	if bits != 16 || (channels != 1 && channels != 2) || sampleRate <= 0 {
		return nil, 0, 0, 0, errors.New("unsupported WAV: need 16-bit mono/stereo PCM")
	}
	if st, err := f.Stat(); err == nil && dataSize > st.Size()-44 {
		dataSize = st.Size() - 44
	}

	// Section starts at PCM data (byte 44), so Seek offsets exclude the header.
	return io.NewSectionReader(f, 44, dataSize), sampleRate, channels, dataSize, nil
}

const outRate = 44100

// normalize converts any 16-bit mono/stereo PCM stream to 44100Hz stereo (the Oto context format).
func normalize(src io.ReadSeeker, rate, ch int, length int64) *DecodedAudio {
	if rate == outRate && ch == 2 {
		return &DecodedAudio{Stream: src, SampleRate: outRate, Channels: 2, Length: length}
	}
	frames := length / int64(ch*2)
	r := &resampler{src: src, br: bufio.NewReader(src), rate: rate, ch: ch, i0: -1}
	return &DecodedAudio{Stream: r, SampleRate: outRate, Channels: 2, Length: frames * outRate / int64(rate) * 4}
}

// resampler: linear-interpolation resampler to 44100Hz stereo s16le.
// ponytail: linear interp is fine for music overlay; use a polyphase filter if aliasing matters.
type resampler struct {
	src      io.ReadSeeker
	br       *bufio.Reader
	rate, ch int
	outFrame int64
	i0       int64 // source frame index held in s0
	s0, s1   [2]int16
	eof      bool
}

func (r *resampler) readFrame() ([2]int16, bool) {
	var b [4]byte
	if _, err := io.ReadFull(r.br, b[:r.ch*2]); err != nil {
		return [2]int16{}, false
	}
	l := int16(binary.LittleEndian.Uint16(b[0:2]))
	if r.ch == 1 {
		return [2]int16{l, l}, true
	}
	return [2]int16{l, int16(binary.LittleEndian.Uint16(b[2:4]))}, true
}

func (r *resampler) Read(p []byte) (int, error) {
	if r.eof {
		return 0, io.EOF
	}
	n := 0
	for n+4 <= len(p) {
		pf := float64(r.outFrame) * float64(r.rate) / outRate
		i := int64(pf)
		frac := pf - float64(i)

		if r.i0 < 0 {
			var ok bool
			if r.s0, ok = r.readFrame(); !ok {
				r.eof = true
				break
			}
			r.i0 = 0
			if r.s1, ok = r.readFrame(); !ok {
				r.s1 = r.s0
				r.eof = true
			}
		}

		for r.i0 < i && !r.eof {
			r.s0 = r.s1
			r.i0++
			var ok bool
			if r.s1, ok = r.readFrame(); !ok {
				r.s1 = r.s0
				r.eof = true
				break
			}
		}

		if r.eof && i > r.i0 {
			break
		}

		for c := 0; c < 2; c++ {
			v := float64(r.s0[c])*(1-frac) + float64(r.s1[c])*frac
			binary.LittleEndian.PutUint16(p[n+c*2:], uint16(int16(v)))
		}
		n += 4
		r.outFrame++
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (r *resampler) Seek(offset int64, whence int) (int64, error) {
	if whence != io.SeekStart {
		return 0, errors.New("resampler: only SeekStart supported")
	}
	outFrame := offset / 4
	srcFrame := int64(float64(outFrame) * float64(r.rate) / outRate)
	if _, err := r.src.Seek(srcFrame*int64(r.ch*2), io.SeekStart); err != nil {
		return 0, err
	}
	r.br.Reset(r.src)
	r.outFrame = outFrame
	r.eof = false
	r.i0 = -1
	return offset, nil
}
