// IslandWin - Windows Dynamic Island Overlay
// M3: Daily To-Do List (atomic JSON storage, carry-over, custom text field, clipboard paste, task CRUD).
package main

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"islandwin/internal/anim"
	"islandwin/internal/audio"
	"islandwin/internal/config"
	"islandwin/internal/prayer"
	"islandwin/internal/render"
	"islandwin/internal/todo"
	"islandwin/internal/ui"
	"islandwin/internal/win"
)

const (
	canvasW = render.CanvasW
	canvasH = render.CanvasH

	timerAnim       uintptr = 1
	timerFullscreen uintptr = 2
	timerProgress   uintptr = 3
	timerCaret      uintptr = 4
)

type App struct {
	cfg       *config.Config
	window    *win.Window
	canvas    *render.Canvas
	machine   *ui.Machine
	springW   *anim.Spring
	springH   *anim.Spring
	animating bool
	tickCount uint64
	slowAnim  bool // timerAnim running at 64ms (collapsed + playing)

	// Playlist view (inside music panel)
	showPlaylist   bool
	playlistOffset int
	lastTick       time.Time

	// Audio subsystem
	library *audio.Library
	player  *audio.Player
	libPath string

	// Todo subsystem
	todoStore        *todo.Store
	todoInputText    string
	todoInputFocused bool
	cursorBlink      bool

	// Prayer reminder (schedule written by prayerLoop goroutine, everything else UI thread only)
	prayerSched  atomic.Pointer[prayer.Schedule]
	prayerCity   atomic.Pointer[prayer.City]
	prayerWake   chan struct{}
	prayerEvents chan prayerEvent
	prayerNext   string // "Maghrib - 17:49"; empty = no label
	prayerHH     string
	prayerMM     string
	prayerUrgent bool
	prayerBlink  bool
	alertActive  bool
	alertUntil   time.Time
	alertTitle   string
	alertSub     string
	lastAlertKey string

	// Hover timing
	mouseInside  bool
	hoverEnterAt time.Time
	hoverLeaveAt time.Time

	suppressedByFullscreen bool
}

func main() {
	mutex, err := win.TryAcquireSingleInstance("IslandWin_SingleInstance_Mutex")
	if err != nil {
		fmt.Println("IslandWin is already running.")
		os.Exit(0)
	}
	defer mutex.Release()

	// Memory limits (PRD §7.2)
	debug.SetGCPercent(50)
	debug.SetMemoryLimit(64 << 20)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)
	slog.Info("IslandWin M3 starting")

	cfg, err := config.Load()
	if err != nil {
		slog.Warn("Failed to load config, using defaults", "err", err)
	}

	appDataDir, _ := config.AppDataDir()
	libPath := filepath.Join(appDataDir, "library.json")
	todoPath := filepath.Join(appDataDir, "todos.json")

	lib, err := audio.LoadLibrary(libPath)
	if err != nil {
		lib = audio.NewLibrary()
	}

	player, err := audio.NewPlayer(lib)
	if err != nil {
		slog.Warn("Audio initialization failed", "err", err)
	} else if cfg.LastPlayed.Volume > 0 {
		player.SetVolume(cfg.LastPlayed.Volume)
	}

	todoStore, err := todo.NewStore(todoPath, cfg.CarryOverTodos)
	if err != nil {
		slog.Warn("Failed to load todo store", "err", err)
	}

	app := &App{
		cfg:       cfg,
		canvas:    render.New(canvasW, canvasH),
		machine:   ui.NewMachine(),
		springW:   anim.New(render.CollapsedW),
		springH:   anim.New(render.CollapsedH),
		library:   lib,
		player:    player,
		libPath:   libPath,
		todoStore: todoStore,
	}

	app.window, err = win.NewWindow(canvasW, canvasH)
	if err != nil {
		slog.Error("failed to create window", "err", err)
		os.Exit(1)
	}

	city := prayer.City{ID: cfg.PrayerCityID, Name: cfg.PrayerCityName}
	if city.ID == "" {
		city = prayer.DefaultCity
	}
	app.prayerCity.Store(&city)
	app.prayerWake = make(chan struct{}, 1)
	app.prayerEvents = make(chan prayerEvent, 4)
	app.window.PrayerCityLabel = city.Name
	app.window.PrayerCityChoices = prayerCities

	_ = app.window.AddTrayIcon("IslandWin - Dynamic Island")
	_ = app.window.RegisterToggleHotkey()
	app.window.RegisterMediaHotkeys()

	// Connect window callbacks
	app.window.OnMouseEnter = app.onMouseEnter
	app.window.OnMouseLeave = app.onMouseLeave
	app.window.OnClick = app.onClick
	app.window.OnEscape = app.onEscape
	app.window.OnHotkey = app.onHotkey
	app.window.OnChar = app.onChar
	app.window.OnKeyDown = app.onKeyDown
	app.window.OnMediaPlayPause = app.onMediaPlayPause
	app.window.OnMediaNext = app.onMediaNext
	app.window.OnMediaPrev = app.onMediaPrev
	app.window.OnTick = app.onTick
	app.window.OnDisplayChange = app.onDisplayChange
	app.window.OnTrayToggle = app.onTrayToggle
	app.window.OnTraySelectFolder = app.onTraySelectFolder
	app.window.OnTrayAutostartToggle = app.onTrayAutostartToggle
	app.window.OnTrayExit = app.onTrayExit
	app.window.OnTrayPrayerSync = app.onPrayerSync
	app.window.OnTrayPrayerClipboard = app.onPrayerClipboard
	app.window.OnTrayPrayerCity = app.findCityAsync

	// Listen to player state changes
	// ponytail: callbacks fire on audio goroutines; Win32 timers and canvas are UI-thread only.
	// UI redraws come from timerAnim (runs while playing), so callbacks stay no-ops.
	if app.player != nil {
		app.player.OnStateChange = func(isPlaying bool) {}
		app.player.OnTrackChange = func(t audio.Track) {}
	}

	// Scan configured music folders in background (F-MUS-02)
	go app.scanFoldersInBackground()

	if cfg.PrayerAlerts {
		go app.prayerLoop(filepath.Join(appDataDir, "prayer.json"))
	}

	// Check fullscreen periodically (1s)
	app.window.StartTimer(timerFullscreen, 1000)

	// Draw initial state
	app.drawAndPresent()

	ramMB := win.GetWorkingSetMB()
	slog.Info("IslandWin M3 running", "working_set_MB", fmt.Sprintf("%.2f", ramMB))
	fmt.Printf("\n=== IslandWin M3 ===\n")
	fmt.Printf("RAM (working set): %.2f MB\n", ramMB)
	fmt.Printf("Target: < 30 MB idle, < 60 MB playing\n")
	fmt.Printf("Tracks in Library: %d\n", lib.Count())
	fmt.Printf("Active Tasks: %d\n", todoStore.IncompleteCount())
	fmt.Printf("Status: OK\n")
	fmt.Printf("====================\n\n")

	app.window.RunMessageLoop()

	// Cleanup on exit
	app.saveState()
	if app.player != nil {
		_ = app.player.Close()
	}
	debug.FreeOSMemory()
}

// prayerCities feeds the tray's manual picker; each name is resolved to a city ID via the API.
var prayerCities = []string{
	"Jakarta", "Bogor", "Depok", "Tangerang", "Bekasi", "Bandung", "Semarang", "Yogyakarta",
	"Surabaya", "Malang", "Medan", "Palembang", "Makassar", "Balikpapan", "Denpasar",
}

// prayerEvent is the result of an async city lookup, consumed on the UI thread.
type prayerEvent struct {
	city *prayer.City
	msg  string
}

func (a *App) sendPrayerEvent(c prayer.City, err error) {
	ev := prayerEvent{city: &c}
	if err != nil {
		ev = prayerEvent{msg: "Gagal terhubung ke server"}
		if err == prayer.ErrNotFound {
			ev.msg = "Kota tidak ditemukan"
		}
	}
	select {
	case a.prayerEvents <- ev:
	default:
	}
}

// findCityAsync resolves a city by keyword off the UI thread.
func (a *App) findCityAsync(keyword string) {
	go func() {
		c, err := prayer.FindCity(keyword)
		a.sendPrayerEvent(c, err)
	}()
}

// onPrayerSync guesses the city from the public IP, then resolves it.
func (a *App) onPrayerSync() {
	go func() {
		name, err := prayer.GeoCity()
		if err != nil {
			a.sendPrayerEvent(prayer.City{}, err)
			return
		}
		c, err := prayer.FindCity(name)
		a.sendPrayerEvent(c, err)
	}()
}

// onPrayerClipboard treats the first clipboard line as a city name (manual input for any city).
func (a *App) onPrayerClipboard() {
	txt, _ := win.GetClipboardText(uintptr(a.window.Hwnd))
	kw, _, _ := strings.Cut(strings.TrimSpace(txt), "\n")
	if kw = strings.TrimSpace(kw); kw == "" {
		a.showAlert("Clipboard kosong", "Salin nama kota dulu, mis. Bandung", 4*time.Second, time.Now())
		return
	}
	a.findCityAsync(kw)
}

// prayerLoop refreshes the schedule; prayer.Get is cached per day+city so this is one HTTP call/day.
func (a *App) prayerLoop(path string) {
	for {
		wait := 5 * time.Minute
		c := a.prayerCity.Load()
		if s, err := prayer.Get(path, c.ID, time.Now()); err == nil {
			// Drop the result if the user switched city while the request was in flight.
			if cur := a.prayerCity.Load(); cur.ID == s.CityID {
				a.prayerSched.Store(s)
			}
		} else {
			slog.Warn("prayer schedule fetch failed", "err", err)
			wait = 2 * time.Minute
		}
		select {
		case <-time.After(wait):
		case <-a.prayerWake:
		}
	}
}

// showAlert shows a banner on the collapsed island. Skipped while a panel is open.
func (a *App) showAlert(title, sub string, d time.Duration, now time.Time) {
	if a.machine.Current != ui.StateCollapsed {
		return
	}
	a.alertTitle, a.alertSub = title, sub
	a.alertActive = true
	a.alertUntil = now.Add(d)
	a.applyTargetFromMachine()
	a.window.StartTimer(timerAnim, 16)
}

func (a *App) applyPrayerEvent(ev prayerEvent, now time.Time) {
	if ev.city == nil {
		a.showAlert(ev.msg, "", 4*time.Second, now)
		return
	}
	a.prayerCity.Store(ev.city)
	a.cfg.PrayerCityID, a.cfg.PrayerCityName = ev.city.ID, ev.city.Name
	_ = a.cfg.Save()
	a.window.PrayerCityLabel = ev.city.Name
	a.prayerSched.Store(nil)
	a.prayerNext = ""
	select {
	case a.prayerWake <- struct{}{}:
	default:
	}
	a.showAlert("Kota sholat diganti", ev.city.Name, 4*time.Second, now)
}

// checkPrayer runs on the UI thread every second. Alerts only when island is idle (collapsed, no hover);
// otherwise retries until the 3-minute due window expires.
func (a *App) checkPrayer(now time.Time) {
	for drained := false; !drained; {
		select {
		case ev := <-a.prayerEvents:
			a.applyPrayerEvent(ev, now)
		default:
			drained = true
		}
	}

	if a.alertActive && now.After(a.alertUntil) {
		a.alertActive = false
		a.applyTargetFromMachine()
		a.window.StartTimer(timerAnim, 16)
	}

	if a.machine.Current == ui.StateExpandedPrayer {
		a.drawAndPresent()
	}

	s := a.prayerSched.Load()
	if s == nil {
		a.prayerNext = ""
		a.syncCollapsedW()
		return
	}
	old := a.prayerNext + a.prayerHH + a.prayerMM
	if n, h, t, _, ok := s.NextWithTarget(now); ok {
		d := t.Sub(now)
		lead := time.Duration(a.cfg.PrayerRemindMin) * time.Minute
		if lead <= 0 {
			lead = 15 * time.Minute
		}
		a.prayerNext = n + " - " + h
		a.prayerHH, a.prayerMM = prayer.Remaining(d)
		a.prayerUrgent = d <= lead
	} else {
		a.prayerNext, a.prayerHH, a.prayerMM, a.prayerUrgent = "", "", "", false
	}
	a.prayerBlink = now.Second()%2 == 0
	a.syncCollapsedW()
	// Timer anim stops when idle: redraw on label change (about once a minute) and every second while blinking.
	if (a.prayerNext+a.prayerHH+a.prayerMM != old || a.prayerUrgent) && a.machine.Current == ui.StateCollapsed && !a.suppressedByFullscreen {
		a.drawAndPresent()
	}

	if a.alertActive || a.machine.Current != ui.StateCollapsed || a.mouseInside {
		return
	}
	city := a.prayerCity.Load().Name
	if i, ok := s.Due(now); ok {
		key := s.Date + prayer.Names[i]
		if key == a.lastAlertKey {
			return
		}
		a.lastAlertKey = key
		win.Beep()
		a.showAlert("Waktu Sholat "+prayer.Names[i], s.Times[i]+" · "+city, 10*time.Second, now)
		return
	}
	// Pre-reminder; retried each second until prayer time while island is busy.
	if lead := time.Duration(a.cfg.PrayerRemindMin) * time.Minute; lead > 0 {
		if i, ok := s.Soon(now, lead); ok {
			key := "pre" + s.Date + prayer.Names[i]
			if key == a.lastAlertKey {
				return
			}
			a.lastAlertKey = key
			t, _ := time.ParseInLocation("2006-01-02 15:04", s.Date+" "+s.Times[i], prayer.Zone)
			mins := int(t.Sub(now)/time.Minute) + 1
			a.showAlert(fmt.Sprintf("%s %d menit lagi", prayer.Names[i], mins), s.Times[i]+" · "+city, 8*time.Second, now)
		}
	}
}

func (a *App) scanFoldersInBackground() {
	if len(a.cfg.MusicFolders) == 0 {
		return
	}
	for _, folder := range a.cfg.MusicFolders {
		a.library.ScanFolder(folder)
	}
	_ = a.library.Save(a.libPath)
	debug.FreeOSMemory()
	a.drawAndPresent()
}

func (a *App) saveState() {
	if a.player != nil {
		status := a.player.GetStatus()
		a.cfg.LastPlayed.Volume = status.Volume
		a.cfg.LastPlayed.Path = status.CurrentTrack.Path
		a.cfg.LastPlayed.PositionMs = int64(status.PositionSec * 1000)
	}
	_ = a.cfg.Save()
	_ = a.library.Save(a.libPath)
	_ = a.todoStore.Save()
}

func (a *App) onMouseEnter() {
	a.mouseInside = true
	a.hoverEnterAt = time.Now()
	a.window.StartTimer(timerAnim, 16)
}

func (a *App) onMouseLeave() {
	a.mouseInside = false
	a.hoverLeaveAt = time.Now()
	a.window.StartTimer(timerAnim, 16)
}

func (a *App) onChar(r rune) {
	if !a.todoInputFocused {
		return
	}
	// Printable Unicode characters (F-TODO-08)
	if r >= 32 && r != 127 {
		a.todoInputText += string(r)
		a.cursorBlink = true
		a.drawAndPresent()
	}
}

func (a *App) onKeyDown(vk uint32, ctrl bool) {
	if !a.todoInputFocused {
		return
	}

	switch vk {
	case win.VK_RETURN:
		// Submit task
		if a.todoInputText != "" {
			a.todoStore.Add(a.todoInputText)
			a.todoInputText = ""
			a.drawAndPresent()
		}
	case win.VK_BACK:
		// Backspace
		runes := []rune(a.todoInputText)
		if len(runes) > 0 {
			a.todoInputText = string(runes[:len(runes)-1])
			a.cursorBlink = true
			a.drawAndPresent()
		}
	case win.VK_DELETE:
		a.todoInputText = ""
		a.drawAndPresent()
	case 0x56: // 'V' key
		if ctrl {
			// Paste from clipboard (F-TODO-08)
			clip, err := win.GetClipboardText(uintptr(a.window.Hwnd))
			if err == nil && clip != "" {
				a.todoInputText += clip
				a.cursorBlink = true
				a.drawAndPresent()
			}
		}
	}
}

func (a *App) onClick(x, y float64) {
	// Click on prayer alert dismisses it.
	if a.alertActive {
		a.alertActive = false
		a.applyTargetFromMachine()
		a.window.StartTimer(timerAnim, 16)
		return
	}

	// Clicking while collapsed expands and pins (F-ISL-04).
	if a.machine.Current == ui.StateCollapsed {
		a.machine.TogglePin()
		a.applyTargetFromMachine()
		a.window.StartTimer(timerAnim, 16)
		return
	}

	pillW := math.Round(a.springW.Value)
	pillH := math.Round(a.springH.Value)

	// Interactive hit-test for Music Panel
	if a.machine.Current == ui.StateExpandedMusic {
		if a.showPlaylist {
			a.onPlaylistClick(x, y, pillW)
			return
		}

		rects := render.GetMusicUIRects(float64(canvasW), pillW, pillH)

		if rects.Playlist.Contains(x, y) {
			a.showPlaylist = true
			cur := -1
			if a.player != nil {
				cur = a.player.GetStatus().CurrentIndex
			}
			a.playlistOffset = cur - 2
			a.clampPlaylist()
			a.applyTargetFromMachine()
			a.window.StartTimer(timerAnim, 16)
			return
		}

		if rects.PlayPause.Contains(x, y) {
			if a.player != nil {
				a.player.TogglePlayPause()
				if a.player.GetStatus().IsPlaying {
					a.window.StartTimer(timerAnim, 16)
				}
				a.drawAndPresent()
			}
			return
		}

		if rects.Next.Contains(x, y) {
			if a.player != nil {
				a.player.Next()
				a.window.StartTimer(timerAnim, 16)
				a.drawAndPresent()
			}
			return
		}

		if rects.Prev.Contains(x, y) {
			if a.player != nil {
				a.player.Previous()
				a.window.StartTimer(timerAnim, 16)
				a.drawAndPresent()
			}
			return
		}

		if rects.SelectFolder.Contains(x, y) || (a.library.Count() == 0 && rects.AlbumArt.Contains(x, y)) {
			a.onTraySelectFolder()
			return
		}

		if rects.ProgressBar.Contains(x, y) {
			if a.player != nil && rects.ProgressBar.W > 0 {
				ratio := (x - rects.ProgressBar.X) / rects.ProgressBar.W
				a.player.Seek(ratio)
				a.drawAndPresent()
			}
			return
		}

		if rects.VolumeBar.Contains(x, y) {
			if a.player != nil && rects.VolumeBar.W > 0 {
				vol := (x - rects.VolumeBar.X) / rects.VolumeBar.W
				a.player.SetVolume(vol)
				a.drawAndPresent()
			}
			return
		}

		if rects.SwitchTab.Contains(x, y) {
			a.machine.SwitchExpanded()
			a.applyTargetFromMachine()
			a.window.StartTimer(timerAnim, 16)
			return
		}

		// ponytail: klik area kosong dulu toggle pin => island sering nyangkut full. Pin kini hanya via klik saat collapsed/hotkey; Esc/hotkey melepas.
		return
	}

	// Interactive hit-test for To-Do Panel (F-TODO-02, F-TODO-03)
	if a.machine.Current == ui.StateExpandedTodo {
		items := a.todoStore.GetItems()
		rects := render.GetTodoUIRects(float64(canvasW), pillW, pillH, items)

		// Switch tab button
		if rects.SwitchTab.Contains(x, y) {
			a.unfocusTodoInput()
			a.machine.SwitchExpanded()
			a.applyTargetFromMachine()
			a.window.StartTimer(timerAnim, 16)
			return
		}

		// Check item clicks (checkbox or delete)
		for _, ir := range rects.Items {
			if ir.Checkbox.Contains(x, y) {
				a.todoStore.Toggle(ir.ID)
				a.drawAndPresent()
				return
			}
			if ir.Delete.Contains(x, y) {
				a.todoStore.Delete(ir.ID)
				a.drawAndPresent()
				return
			}
		}

		// Input box click: focus and enable caret
		if rects.InputBox.Contains(x, y) {
			a.todoInputFocused = true
			a.cursorBlink = true
			a.window.Focus()
			a.window.StartTimer(timerCaret, 500)
			a.drawAndPresent()
			return
		}

		// Click outside input box unfocuses it
		a.unfocusTodoInput()
	}

	if a.machine.Current == ui.StateExpandedPrayer {
		if render.GetPrayerUIRects(float64(canvasW), pillW).SwitchTab.Contains(x, y) {
			a.machine.SwitchExpanded()
			a.applyTargetFromMachine()
			a.window.StartTimer(timerAnim, 16)
		}
	}
}

// prayerPanelData builds countdown + rows for the Sholat panel.
// ponytail: after Isya no tomorrow schedule is fetched; shows "Selesai". Fetch next day's Subuh if wanted.
func (a *App) prayerPanelData(now time.Time) (city, nextName, countdown string, rows []render.PrayerRow) {
	if c := a.prayerCity.Load(); c != nil {
		city = c.Name
	}
	s := a.prayerSched.Load()
	if s == nil {
		return city, "Memuat jadwal...", "", nil
	}
	nextIdx := -1
	if n, _, t, i, ok := s.NextWithTarget(now); ok {
		nextIdx = i
		d := t.Sub(now).Round(time.Second)
		nextName = n
		countdown = fmt.Sprintf("%02d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
	} else {
		nextName, countdown = "Selesai", "hingga Subuh besok"
	}
	for i, name := range prayer.Names {
		rows = append(rows, render.PrayerRow{Name: name, Time: s.Times[i], Next: i == nextIdx, Past: s.Passed(i, now)})
	}
	return
}

func (a *App) clampPlaylist() {
	max := a.library.Count() - render.PlaylistRows
	if max < 0 {
		max = 0
	}
	if a.playlistOffset > max {
		a.playlistOffset = max
	}
	if a.playlistOffset < 0 {
		a.playlistOffset = 0
	}
}

func (a *App) onPlaylistClick(x, y, pillW float64) {
	r := render.GetPlaylistUIRects(float64(canvasW), pillW)
	switch {
	case r.Back.Contains(x, y):
		a.showPlaylist = false
		a.applyTargetFromMachine()
		a.window.StartTimer(timerAnim, 16)
	case r.Up.Contains(x, y):
		a.playlistOffset -= render.PlaylistRows - 1
		a.clampPlaylist()
		a.drawAndPresent()
	case r.Down.Contains(x, y):
		a.playlistOffset += render.PlaylistRows - 1
		a.clampPlaylist()
		a.drawAndPresent()
	default:
		total := a.library.Count()
		for i, row := range r.Rows {
			idx := a.playlistOffset + i
			if idx >= total || !row.Contains(x, y) {
				continue
			}
			if a.player != nil {
				if err := a.player.PlayIndex(idx); err != nil {
					slog.Warn("PlayIndex failed", "idx", idx, "err", err)
				}
				a.window.StartTimer(timerAnim, 16)
				a.drawAndPresent()
			}
			return
		}
	}
}

func (a *App) unfocusTodoInput() {
	if a.todoInputFocused {
		a.todoInputFocused = false
		a.cursorBlink = false
		a.window.StopTimer(timerCaret)
		a.drawAndPresent()
	}
}

func (a *App) onEscape() {
	if a.todoInputFocused {
		a.unfocusTodoInput()
		return
	}
	if _, changed := a.machine.UnpinAndCollapse(); changed {
		a.applyTargetFromMachine()
		a.window.StartTimer(timerAnim, 16)
	}
}

func (a *App) onHotkey() {
	a.machine.ToggleExpand()
	a.applyTargetFromMachine()
	a.window.StartTimer(timerAnim, 16)
}

func (a *App) onMediaPlayPause() {
	if a.player != nil {
		a.player.TogglePlayPause()
		if a.player.GetStatus().IsPlaying {
			a.window.StartTimer(timerAnim, 16)
		}
		a.drawAndPresent()
	}
}

func (a *App) onMediaNext() {
	if a.player != nil {
		a.player.Next()
		a.window.StartTimer(timerAnim, 16)
		a.drawAndPresent()
	}
}

func (a *App) onMediaPrev() {
	if a.player != nil {
		a.player.Previous()
		a.window.StartTimer(timerAnim, 16)
		a.drawAndPresent()
	}
}

func (a *App) onTrayToggle() {
	a.machine.ToggleExpand()
	a.applyTargetFromMachine()
	a.window.StartTimer(timerAnim, 16)
}

func (a *App) onTraySelectFolder() {
	folder, err := win.SelectFolderDialog(uintptr(a.window.Hwnd), "Pilih Folder Koleksi Musik")
	if err != nil || folder == "" {
		return
	}

	exists := false
	for _, f := range a.cfg.MusicFolders {
		if f == folder {
			exists = true
			break
		}
	}
	if !exists {
		a.cfg.MusicFolders = append(a.cfg.MusicFolders, folder)
		_ = a.cfg.Save()
	}

	go func() {
		count := a.library.ScanFolder(folder)
		_ = a.library.Save(a.libPath)
		slog.Info("Music folder scanned", "folder", folder, "tracks_found", count)
		debug.FreeOSMemory()
		a.drawAndPresent()
	}()
}

func (a *App) onTrayAutostartToggle() {
	current := win.IsAutostartEnabled()
	newState := !current
	if err := win.SetAutostart(newState); err != nil {
		slog.Error("Failed to toggle autostart", "err", err)
	} else {
		slog.Info("Autostart setting updated", "enabled", newState)
	}
}

func (a *App) onTrayExit() {
	a.window.Close()
}

func (a *App) onDisplayChange() {
	a.window.Reposition()
	a.drawAndPresent()
}

func (a *App) onTick(id uintptr) {
	// Caret blinks only on its own 500ms timer. Before, every 16ms anim tick toggled it => flicker.
	if id == timerCaret {
		if a.todoInputFocused {
			a.cursorBlink = !a.cursorBlink
			a.drawAndPresent()
		}
		return
	}

	now := time.Now()
	dt := 1.0 / 60.0
	if !a.lastTick.IsZero() {
		actualDt := now.Sub(a.lastTick).Seconds()
		if actualDt > 0.001 && actualDt < 0.050 {
			dt = actualDt
		}
	}
	a.lastTick = now
	if a.slowAnim {
		a.tickCount += 4 // keep equalizer speed at 4x slower tick rate
	} else {
		a.tickCount++
	}

	// Fullscreen auto-hide check
	if win.IsFullscreenAppActive() {
		if !a.suppressedByFullscreen {
			a.suppressedByFullscreen = true
			a.window.SetVisible(false)
		}
		return
	} else if a.suppressedByFullscreen {
		a.suppressedByFullscreen = false
		a.window.SetVisible(true)
		a.drawAndPresent()
	}

	// User klik app lain: lepas fokus input supaya island bisa auto-collapse.
	if a.todoInputFocused && !a.window.IsForeground() {
		a.unfocusTodoInput()
	}

	// Keep mouseInside state synchronized with actual cursor position
	if a.window.IsCursorInsidePill() {
		if !a.mouseInside {
			a.mouseInside = true
			a.hoverEnterAt = now
		}
	} else if a.mouseInside {
		a.mouseInside = false
		a.hoverLeaveAt = now
	}

	if id == timerFullscreen {
		a.checkPrayer(now)
	}

	// Hover logic
	if a.mouseInside && a.machine.Current == ui.StateCollapsed && !a.alertActive {
		if now.Sub(a.hoverEnterAt) >= time.Duration(a.cfg.HoverOpenMs)*time.Millisecond {
			a.machine.HoverEnter()
			a.applyTargetFromMachine()
		}
	} else if !a.mouseInside && !a.machine.Pinned && a.machine.Current != ui.StateCollapsed && !a.todoInputFocused {
		if now.Sub(a.hoverLeaveAt) >= time.Duration(a.cfg.HoverCloseMs)*time.Millisecond {
			a.machine.HoverLeave()
			a.applyTargetFromMachine()
		}
	}

	// Advance spring animation
	movingW := a.springW.Tick(dt)
	movingH := a.springH.Tick(dt)
	a.animating = movingW || movingH

	a.drawAndPresent()

	isPlaying := false
	if a.player != nil {
		isPlaying = a.player.GetStatus().IsPlaying
	}

	if !a.animating && !a.mouseInside && (a.machine.Pinned || a.machine.Current == ui.StateCollapsed) && !a.todoInputFocused {
		if !isPlaying {
			a.window.StopTimer(timerAnim)
			a.slowAnim = false
		}
	}

	// Collapsed + playing: only the equalizer moves, so ~15fps is enough (was 60fps full redraw).
	slow := isPlaying && !a.animating && !a.mouseInside && !a.alertActive && !a.todoInputFocused &&
		a.machine.Current == ui.StateCollapsed
	if slow != a.slowAnim {
		a.slowAnim = slow
		if slow {
			a.window.StartTimer(timerAnim, 64)
		} else {
			a.window.StartTimer(timerAnim, 16)
		}
	}
}

// collapsedW widens the collapsed pill while music plays so disc, prayer label and equalizer don't overlap.
func (a *App) collapsedW() float64 {
	if a.prayerNext == "" {
		return render.CollapsedW
	}
	if a.player != nil && a.player.GetStatus().IsPlaying {
		return render.CollapsedPlayingW
	}
	return render.CollapsedPrayerW
}

// syncCollapsedW re-targets the collapsed pill when play state or label presence changed.
func (a *App) syncCollapsedW() {
	if a.machine.Current == ui.StateCollapsed && !a.alertActive && a.springW.Target != a.collapsedW() {
		a.applyTargetFromMachine()
		a.window.StartTimer(timerAnim, 16)
	}
}

func (a *App) applyTargetFromMachine() {
	tw, th := a.machine.Current.Dimensions()
	if a.machine.Current == ui.StateCollapsed {
		tw = a.collapsedW()
	}
	if a.alertActive && a.machine.Current == ui.StateCollapsed {
		tw, th = render.AlertW, render.AlertH
	}
	// Fokus input to-do yang tertinggal memblokir auto-collapse.
	if a.machine.Current == ui.StateCollapsed && a.todoInputFocused {
		a.todoInputFocused = false
		a.cursorBlink = false
		a.window.StopTimer(timerCaret)
	}
	if a.machine.Current != ui.StateExpandedMusic {
		a.showPlaylist = false
	} else if a.showPlaylist {
		th = render.PlaylistH
	}
	a.springW.SetTarget(tw)
	a.springH.SetTarget(th)
	a.animating = true
}

func (a *App) drawAndPresent() {
	pillW := math.Round(a.springW.Value)
	pillH := math.Round(a.springH.Value)

	a.window.PillW = pillW
	a.window.PillH = pillH

	a.canvas.Clear()
	a.canvas.DrawPill(pillW, pillH)

	isPlaying := false
	if a.player != nil {
		isPlaying = a.player.GetStatus().IsPlaying
	}

	// Collapsed state mini-indicators (mini disc + animated equalizer when playing)
	baseW := a.collapsedW()
	a.canvas.DrawCollapsedIndicator(pillW, pillH, isPlaying, a.tickCount, baseW)
	a.canvas.DrawNextPrayer(pillW, pillH, a.prayerNext, a.prayerHH, a.prayerMM, baseW, a.prayerUrgent, a.prayerBlink)
	if a.alertActive {
		a.canvas.DrawPrayerAlert(pillW, pillH, a.alertTitle, a.alertSub)
	}

	switch a.machine.Current {
	case ui.StateExpandedMusic:
		title := "Belum ada lagu"
		artist := "Klik kanan tray -> Pilih Folder Musik"
		posSec := 0.0
		durSec := 0.0
		vol := 0.8

		if a.player != nil {
			st := a.player.GetStatus()
			posSec = st.PositionSec
			durSec = st.DurationSec
			vol = st.Volume
			if st.CurrentTrack.Title != "" {
				title = st.CurrentTrack.Title
				artist = st.CurrentTrack.Artist
			} else if a.library.Count() > 0 {
				t, _ := a.library.GetTrack(0)
				title = t.Title
				artist = t.Artist
			}
		}
		hasTracks := a.library.Count() > 0
		if a.showPlaylist {
			a.clampPlaylist()
			cur := -1
			if a.player != nil {
				cur = a.player.GetStatus().CurrentIndex
			}
			items := make([]render.PlaylistItem, 0, render.PlaylistRows)
			for i := 0; i < render.PlaylistRows; i++ {
				t, ok := a.library.GetTrack(a.playlistOffset + i)
				if !ok {
					break
				}
				items = append(items, render.PlaylistItem{Title: t.Title, Artist: t.Artist})
			}
			a.canvas.DrawPlaylist(pillW, pillH, items, a.playlistOffset, a.library.Count(), cur)
		} else {
			a.canvas.DrawMusicPlayer(pillW, pillH, title, artist, isPlaying, posSec, durSec, vol, hasTracks)
		}

	case ui.StateExpandedTodo:
		items := a.todoStore.GetItems()
		a.canvas.DrawTodoPanel(pillW, pillH, items, a.todoInputText, a.todoInputFocused, a.cursorBlink)

	case ui.StateExpandedPrayer:
		city, name, cd, rows := a.prayerPanelData(time.Now())
		a.canvas.DrawPrayerPanel(pillW, pillH, city, name, cd, rows)
	}

	bgra := a.canvas.BGRA()
	if err := a.window.Present(bgra); err != nil {
		slog.Error("Present failed", "err", err)
	}
}
