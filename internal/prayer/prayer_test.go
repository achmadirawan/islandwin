package prayer

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScheduleNextDue(t *testing.T) {
	s := &Schedule{Date: "2026-10-29", Times: [5]string{"04:10", "11:40", "14:53", "17:49", "19:00"}}
	at := func(hm string) time.Time {
		x, _ := time.ParseInLocation("2006-01-02 15:04", "2026-10-29 "+hm, Zone)
		return x
	}
	if n, h, ok := s.Next(at("12:00")); !ok || n != "Ashar" || h != "14:53" {
		t.Fatalf("Next = %q %q %v", n, h, ok)
	}
	if n, h, tg, idx, ok := s.NextWithTarget(at("12:00")); !ok || n != "Ashar" || h != "14:53" || idx != 2 || !tg.Equal(at("14:53")) {
		t.Fatalf("NextWithTarget = %q %q %v %d %v", n, h, tg, idx, ok)
	}
	if !s.Passed(0, at("12:00")) || !s.Passed(1, at("12:00")) || s.Passed(2, at("12:00")) {
		t.Fatal("Passed flags wrong at 12:00")
	}
	if _, _, ok := s.Next(at("19:30")); ok {
		t.Fatal("expected no next after Isya")
	}
	if i, ok := s.Due(at("11:41")); !ok || Names[i] != "Dzuhur" {
		t.Fatalf("Due = %d %v", i, ok)
	}
	if _, ok := s.Due(at("11:44")); ok {
		t.Fatal("Due must expire after 3 minutes")
	}
}

func TestSoonAndFormat(t *testing.T) {
	s := &Schedule{Date: "2026-10-29", Times: [5]string{"04:10", "11:40", "14:53", "17:49", "19:00"}}
	at := func(hm string) time.Time {
		x, _ := time.ParseInLocation("2006-01-02 15:04", "2026-10-29 "+hm, Zone)
		return x
	}
	if i, ok := s.Soon(at("14:40"), 15*time.Minute); !ok || i != 2 {
		t.Fatalf("Soon = %d %v", i, ok)
	}
	if _, ok := s.Soon(at("14:30"), 15*time.Minute); ok {
		t.Fatal("too early for reminder")
	}
	if _, ok := s.Soon(at("14:53"), 15*time.Minute); ok {
		t.Fatal("at prayer time is Due, not Soon")
	}
	for d, want := range map[time.Duration]string{
		2*time.Hour + 15*time.Minute + 5*time.Second: "02:16",
		35 * time.Minute: "00:35",
		30 * time.Second: "00:01",
	} {
		hh, mm := Remaining(d)
		if got := hh + ":" + mm; got != want {
			t.Fatalf("Remaining(%v) = %q, want %q", d, got, want)
		}
	}
}

func fakeAPI(t *testing.T, hits *int) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/jadwal/"):
			*hits++
			w.Write([]byte(`{"status":true,"data":{"jadwal":{"subuh":"04:10","dzuhur":"11:40","ashar":"14:53","maghrib":"17:49","isya":"19:00"}}}`))
		case strings.Contains(r.URL.Path, "/kota/cari/"):
			w.Write([]byte(`{"status":true,"data":[{"id":"1201","lokasi":"KAB. BANDUNG"},{"id":"1219","lokasi":"KOTA BANDUNG"}]}`))
		case r.URL.Path == "/geo":
			w.Write([]byte(`{"success":true,"city":"Bandung"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	oldB, oldG := baseURL, geoURL
	baseURL, geoURL = srv.URL, srv.URL+"/geo"
	t.Cleanup(func() { baseURL, geoURL = oldB, oldG })
}

func TestGetFetchesOncePerCityThenCaches(t *testing.T) {
	hits := 0
	fakeAPI(t, &hits)
	path := filepath.Join(t.TempDir(), "prayer.json")
	now := time.Date(2026, 10, 29, 9, 0, 0, 0, Zone)
	for i := 0; i < 2; i++ {
		s, err := Get(path, "1301", now)
		if err != nil || s.Times[1] != "11:40" || s.CityID != "1301" {
			t.Fatalf("Get = %+v, %v", s, err)
		}
	}
	if hits != 1 {
		t.Fatalf("expected 1 HTTP hit, got %d", hits)
	}
	if _, err := Get(path, "1219", now); err != nil || hits != 2 {
		t.Fatalf("city change must refetch: hits=%d err=%v", hits, err)
	}
}

func TestFindCityAndGeo(t *testing.T) {
	hits := 0
	fakeAPI(t, &hits)
	c, err := FindCity("bandung")
	if err != nil || c.ID != "1219" || c.Name != "KOTA BANDUNG" {
		t.Fatalf("FindCity = %+v, %v", c, err)
	}
	if _, err := FindCity("  "); err != ErrNotFound {
		t.Fatalf("empty keyword err = %v", err)
	}
	if g, err := GeoCity(); err != nil || g != "Bandung" {
		t.Fatalf("GeoCity = %q, %v", g, err)
	}
}
