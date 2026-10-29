// Package prayer fetches and caches the daily prayer schedule (Kemenag data via myquran.com).
// Pure Go, no Win32: safe to call from a goroutine.
package prayer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Zone interprets schedule times. They are local to the city, so the machine's zone is used.
// ponytail: assumes the PC is in the selected city's zone; add a per-city zone if that stops holding.
var Zone = time.Local

// Names matches Schedule.Times order.
var Names = [5]string{"Subuh", "Dzuhur", "Ashar", "Maghrib", "Isya"}

var baseURL = "https://api.myquran.com/v2/sholat"

// geoURL resolves the approximate city from the public IP (only called on explicit user sync).
var geoURL = "https://ipwho.is/?fields=success,city"

// ErrNotFound means no city matched the keyword.
var ErrNotFound = errors.New("city not found")

// City is a myquran city ID plus display name.
type City struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DefaultCity is used until the user picks another.
var DefaultCity = City{ID: "1301", Name: "KOTA JAKARTA"}

// Schedule is one day for one city: Date is YYYY-MM-DD, Times are "HH:MM" in Names order.
type Schedule struct {
	CityID string    `json:"cityId"`
	Date   string    `json:"date"`
	Times  [5]string `json:"times"`
}

func (s *Schedule) at(i int) (time.Time, bool) {
	t, err := time.ParseInLocation("2006-01-02 15:04", s.Date+" "+s.Times[i], Zone)
	return t, err == nil
}

func (s *Schedule) valid() bool {
	for i := range s.Times {
		if _, ok := s.at(i); !ok {
			return false
		}
	}
	return true
}

// Next returns the first prayer strictly after now within this schedule's day.
func (s *Schedule) Next(now time.Time) (name, hhmm string, ok bool) {
	n, h, _, _, ok := s.NextWithTarget(now)
	return n, h, ok
}

// NextWithTarget returns the upcoming prayer name, hh:mm, exact parsed time, index, and ok.
func (s *Schedule) NextWithTarget(now time.Time) (name, hhmm string, target time.Time, idx int, ok bool) {
	for i := range s.Times {
		if t, good := s.at(i); good && t.After(now) {
			return Names[i], s.Times[i], t, i, true
		}
	}
	return "", "", time.Time{}, -1, false
}

// Passed reports whether prayer i in this schedule has already passed now.
func (s *Schedule) Passed(i int, now time.Time) bool {
	if t, good := s.at(i); good {
		return now.After(t)
	}
	return false
}

// Due returns the index of a prayer whose time passed within the last 3 minutes.
func (s *Schedule) Due(now time.Time) (int, bool) {
	for i := range s.Times {
		if t, good := s.at(i); good && !now.Before(t) && now.Sub(t) < 3*time.Minute {
			return i, true
		}
	}
	return 0, false
}

// Soon returns the prayer whose time is within lead from now (but not yet reached).
func (s *Schedule) Soon(now time.Time, lead time.Duration) (int, bool) {
	for i := range s.Times {
		if t, good := s.at(i); good && now.Before(t) && t.Sub(now) <= lead {
			return i, true
		}
	}
	return 0, false
}

// Remaining splits d (rounded up to whole minutes) into zero-padded "HH", "MM".
func Remaining(d time.Duration) (hh, mm string) {
	m := int((d + time.Minute - 1) / time.Minute)
	return fmt.Sprintf("%02d", m/60), fmt.Sprintf("%02d", m%60)
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// getJSON GETs u and decodes a size-limited JSON body into v.
func getJSON(u string, v any) error {
	resp, err := httpClient.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prayer api: HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(v)
}

// pickCity chooses the best match: exact "KOTA <kw>", exact name, any "KOTA ...", then first.
func pickCity(list []City, kw string) (City, bool) {
	kw = strings.ToUpper(strings.TrimSpace(kw))
	if len(list) == 0 {
		return City{}, false
	}
	for _, c := range list {
		if c.Name == "KOTA "+kw || c.Name == kw {
			return c, true
		}
	}
	for _, c := range list {
		if strings.HasPrefix(c.Name, "KOTA ") {
			return c, true
		}
	}
	return list[0], true
}

// FindCity searches cities by keyword (e.g. "bandung") and returns the best match.
func FindCity(keyword string) (City, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || len(keyword) > 60 {
		return City{}, ErrNotFound
	}
	var r struct {
		Status bool `json:"status"`
		Data   []struct {
			ID     string `json:"id"`
			Lokasi string `json:"lokasi"`
		} `json:"data"`
	}
	if err := getJSON(baseURL+"/kota/cari/"+url.PathEscape(keyword), &r); err != nil {
		return City{}, err
	}
	list := make([]City, 0, len(r.Data))
	for _, d := range r.Data {
		list = append(list, City{ID: d.ID, Name: strings.ToUpper(d.Lokasi)})
	}
	if c, ok := pickCity(list, keyword); ok && r.Status {
		return c, nil
	}
	return City{}, ErrNotFound
}

// GeoCity returns the city name guessed from the public IP address.
func GeoCity() (string, error) {
	var r struct {
		Success bool   `json:"success"`
		City    string `json:"city"`
	}
	if err := getJSON(geoURL, &r); err != nil {
		return "", err
	}
	if !r.Success || r.City == "" {
		return "", ErrNotFound
	}
	return r.City, nil
}

func fetch(cityID string, day time.Time) (*Schedule, error) {
	d := day.In(Zone)
	u := fmt.Sprintf("%s/jadwal/%s/%04d/%02d/%02d", baseURL, url.PathEscape(cityID), d.Year(), int(d.Month()), d.Day())
	var r struct {
		Status bool `json:"status"`
		Data   struct {
			Jadwal struct {
				Subuh, Dzuhur, Ashar, Maghrib, Isya string
			} `json:"jadwal"`
		} `json:"data"`
	}
	if err := getJSON(u, &r); err != nil {
		return nil, err
	}
	j := r.Data.Jadwal
	s := &Schedule{CityID: cityID, Date: d.Format("2006-01-02"), Times: [5]string{j.Subuh, j.Dzuhur, j.Ashar, j.Maghrib, j.Isya}}
	if !r.Status || !s.valid() {
		return nil, fmt.Errorf("prayer api: invalid response")
	}
	return s, nil
}

// Get returns today's schedule for cityID from the cache file, or fetches and caches it (one request per day per city).
func Get(path, cityID string, now time.Time) (*Schedule, error) {
	today := now.In(Zone).Format("2006-01-02")
	if b, err := os.ReadFile(path); err == nil {
		var s Schedule
		if json.Unmarshal(b, &s) == nil && s.CityID == cityID && s.Date == today && s.valid() {
			return &s, nil
		}
	}
	s, err := fetch(cityID, now)
	if err != nil {
		return nil, err
	}
	if b, err := json.Marshal(s); err == nil {
		tmp := path + ".tmp"
		if os.WriteFile(tmp, b, 0o644) == nil {
			_ = os.Rename(tmp, path)
		}
	}
	return s, nil
}
