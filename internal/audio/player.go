package audio

import (
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/ebitengine/oto/v3"
)

// PlaybackMode configures track progression.
type PlaybackMode int

const (
	ModeSequential PlaybackMode = iota
	ModeShuffle
	ModeRepeatAll
	ModeRepeatOne
)

// PlayerStatus encapsulates the current playback position and state.
type PlayerStatus struct {
	IsPlaying    bool
	CurrentTrack Track
	CurrentIndex int
	PositionSec  float64
	DurationSec  float64
	Volume       float64
	Mode         PlaybackMode
}

// VolumeReader scales 16-bit LE PCM samples by a volume factor (0.0 to 1.0).
type VolumeReader struct {
	source    io.Reader
	volume    *atomic.Value // holds float64
	bytesRead *atomic.Int64
	onEOF     func()
	eofOnce   sync.Once
	buf       []byte
}

func newVolumeReader(r io.Reader, vol *atomic.Value, bytesRead *atomic.Int64, onEOF func()) *VolumeReader {
	return &VolumeReader{
		source:    r,
		volume:    vol,
		bytesRead: bytesRead,
		onEOF:     onEOF,
		buf:       make([]byte, 4096),
	}
}

func (v *VolumeReader) Read(p []byte) (int, error) {
	n, err := v.source.Read(p)
	if n > 0 && v.bytesRead != nil {
		v.bytesRead.Add(int64(n))
	}

	if (err == io.EOF || n == 0) && v.onEOF != nil {
		v.eofOnce.Do(func() {
			go v.onEOF()
		})
	}

	if n <= 0 {
		return n, err
	}

	vol, ok := v.volume.Load().(float64)
	if !ok || vol >= 0.99 {
		return n, err // full volume, pass through
	}
	if vol <= 0.01 {
		// Muted: zero out bytes
		for i := 0; i < n; i++ {
			p[i] = 0
		}
		return n, err
	}

	// Apply 16-bit linear volume scaling
	for i := 0; i+1 < n; i += 2 {
		val := int16(binary.LittleEndian.Uint16(p[i : i+2]))
		scaled := int16(float64(val) * vol)
		binary.LittleEndian.PutUint16(p[i:i+2], uint16(scaled))
	}

	return n, err
}

// Player controls music playback through Oto WASAPI.
type Player struct {
	mu           sync.Mutex
	otoCtx       *oto.Context
	otoPlayer    *oto.Player
	audio        *DecodedAudio
	currentTrack Track
	currentIndex int
	isPlaying    bool
	volumeVal    atomic.Value // float64
	mode         PlaybackMode

	// Seek and position tracking
	bytesRead   atomic.Int64
	totalBytes  int64
	bytesPerSec int64

	// Event callbacks
	OnTrackChange func(t Track)
	OnStateChange func(isPlaying bool)

	library *Library
	history []int
}

// NewPlayer initializes audio context and returns a player.
func NewPlayer(lib *Library) (*Player, error) {
	op := &oto.NewContextOptions{
		SampleRate:   44100,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	}

	otoCtx, readyChan, err := oto.NewContext(op)
	if err != nil {
		return nil, err
	}
	<-readyChan

	p := &Player{
		otoCtx:       otoCtx,
		currentIndex: -1,
		mode:         ModeSequential,
		library:      lib,
		history:      make([]int, 0),
	}
	p.volumeVal.Store(0.8) // Default 80% volume

	return p, nil
}

// PlayTrack starts playback of a track at index.
func (p *Player) PlayIndex(idx int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	track, ok := p.library.GetTrack(idx)
	if !ok {
		return errors.New("track not found")
	}

	return p.loadAndPlayLocked(track, idx)
}

func (p *Player) loadAndPlayLocked(track Track, idx int) error {
	if p.otoPlayer != nil {
		_ = p.otoPlayer.Close()
		p.otoPlayer = nil
	}

	decoded, err := DecodeFile(track.Path)
	if err != nil {
		slog.Warn("Failed to decode audio file", "path", track.Path, "err", err)
		return err
	}

	p.audio = decoded
	p.currentTrack = track
	p.currentIndex = idx
	p.bytesRead.Store(0)
	p.totalBytes = decoded.Length
	// 44100 * 2 channels * 2 bytes = 176400 bytes/sec
	p.bytesPerSec = int64(decoded.SampleRate * decoded.Channels * 2)
	if p.bytesPerSec <= 0 {
		p.bytesPerSec = 44100 * 2 * 2
	}

	volReader := newVolumeReader(decoded.Stream, &p.volumeVal, &p.bytesRead, func() {
		p.Next()
	})
	p.otoPlayer = p.otoCtx.NewPlayer(volReader)
	p.otoPlayer.Play()
	p.isPlaying = true

	if p.OnTrackChange != nil {
		go p.OnTrackChange(track)
	}
	if p.OnStateChange != nil {
		go p.OnStateChange(true)
	}

	return nil
}

// TogglePlayPause switches between play and pause.
func (p *Player) TogglePlayPause() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.otoPlayer == nil {
		if p.library.Count() > 0 {
			if err := p.loadAndPlayLocked(p.library.Tracks[0], 0); err != nil {
				slog.Error("Failed to play track", "track", p.library.Tracks[0].Path, "err", err)
			}
		}
		return
	}

	if p.isPlaying {
		p.otoPlayer.Pause()
		p.isPlaying = false
	} else {
		p.otoPlayer.Play()
		p.isPlaying = true
	}

	if p.OnStateChange != nil {
		go p.OnStateChange(p.isPlaying)
	}
}

// Next advances to the next track according to PlaybackMode.
func (p *Player) Next() {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := p.library.Count()
	if count == 0 {
		return
	}

	var nextIdx int
	switch p.mode {
	case ModeRepeatOne:
		nextIdx = p.currentIndex
	case ModeShuffle:
		if count > 1 {
			for {
				nextIdx = rand.Intn(count)
				if nextIdx != p.currentIndex {
					break
				}
			}
		}
	default: // ModeSequential, ModeRepeatAll
		nextIdx = p.currentIndex + 1
		if nextIdx >= count {
			if p.mode == ModeRepeatAll {
				nextIdx = 0
			} else {
				return // End of queue
			}
		}
	}

	track, ok := p.library.GetTrack(nextIdx)
	if ok {
		_ = p.loadAndPlayLocked(track, nextIdx)
	}
}

// Previous plays previous track or restarts current if played > 3s.
func (p *Player) Previous() {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := p.library.Count()
	if count == 0 {
		return
	}

	prevIdx := p.currentIndex - 1
	if prevIdx < 0 {
		prevIdx = count - 1
	}

	track, ok := p.library.GetTrack(prevIdx)
	if ok {
		_ = p.loadAndPlayLocked(track, prevIdx)
	}
}

// Seek seeks to ratio (0.0 to 1.0) within current track.
func (p *Player) Seek(ratio float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.audio == nil || p.audio.Length <= 0 {
		return
	}

	if ratio < 0 {
		ratio = 0
	} else if ratio > 1 {
		ratio = 1
	}

	targetByte := int64(float64(p.audio.Length) * ratio)
	// Align to 4-byte boundary (16-bit stereo frame)
	targetByte -= targetByte % 4

	_, err := p.audio.Stream.Seek(targetByte, io.SeekStart)
	if err == nil {
		p.bytesRead.Store(targetByte)
	}
}

// SetVolume updates volume (0.0 to 1.0).
func (p *Player) SetVolume(vol float64) {
	if vol < 0 {
		vol = 0
	} else if vol > 1 {
		vol = 1
	}
	p.volumeVal.Store(vol)
}

// GetVolume returns current volume.
func (p *Player) GetVolume() float64 {
	val, ok := p.volumeVal.Load().(float64)
	if !ok {
		return 0.8
	}
	return val
}

// SetMode updates playback mode.
func (p *Player) SetMode(m PlaybackMode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = m
}

// GetStatus returns snapshot of player state.
func (p *Player) GetStatus() PlayerStatus {
	p.mu.Lock()
	defer p.mu.Unlock()

	var posSec, durSec float64
	if p.otoPlayer != nil && p.bytesPerSec > 0 {
		posSec = float64(p.bytesRead.Load()) / float64(p.bytesPerSec)
		durSec = float64(p.totalBytes) / float64(p.bytesPerSec)
	}

	return PlayerStatus{
		IsPlaying:    p.isPlaying,
		CurrentTrack: p.currentTrack,
		CurrentIndex: p.currentIndex,
		PositionSec:  posSec,
		DurationSec:  durSec,
		Volume:       p.GetVolume(),
		Mode:         p.mode,
	}
}

// Close releases Oto audio device resources.
func (p *Player) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.otoPlayer != nil {
		_ = p.otoPlayer.Close()
	}
	return nil
}
