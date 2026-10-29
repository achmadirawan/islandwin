# IslandWin

Windows Dynamic Island overlay — pill kecil di tengah atas layar, terinspirasi Apple Dynamic Island.

## Status Milestone

| Milestone | Status |
|-----------|--------|
| M0 — PoC Window + Animasi | ✅ Done |
| M1 — Core Overlay (DPI, tray, config) | ✅ Done |
| M2 — Music Player | ✅ Done |
| M3 — To-Do List | ✅ Done |
| M4 — Polish & Optimasi | ✅ Done |

## Cara Build

**Requirement:** Go 1.22+, Windows 10/11 x64

```powershell
# Menggunakan script PowerShell helper:
.\scripts\build.ps1 -Target release
.\scripts\build.ps1 -Target debug
.\scripts\build.ps1 -Target test

# Atau manual via go build:
# Debug build (ada console output untuk RAM report)
go build -o bin\islandwin-debug.exe ./cmd/islandwin

# Release build (tanpa console, binary kecil & stripped)
go build -ldflags="-s -w -H=windowsgui" -o bin\islandwin.exe ./cmd/islandwin
```

## Fitur & Kontrol (Lengkap M1 - M4)

- **Hover** pill → expand dengan animasi spring (buka 150ms, tutup 600ms)
- **Klik saat Collapsed** → Pin expand
- **Music Player Terintegrasi (M2)**:
  - Decode MP3 (`go-mp3`) & WAV secara streaming
  - Audio output WASAPI pure Go (`ebitengine/oto/v3`), tanpa CGO
  - Kontrol interaktif via klik mouse: **Play/Pause**, **Next**, **Previous**, **Seek Bar**, dan **Volume Bar**
  - Tombol keyboard hardware: Media Play/Pause, Next Track, Prev Track
  - Dialog pemilihan folder musik native Windows (`SHBrowseForFolderW`) via Tray Menu
  - Background scanning folder rekursif dengan cache indeks `%APPDATA%\IslandWin\library.json`
  - Menyimpan track, volume, dan posisi terakhir di `%APPDATA%\IslandWin\config.json`
- **Daily To-Do List Terintegrasi (M3)**:
  - Penyimpanan atomik berbasis JSON di `%APPDATA%\IslandWin\todos.json`
  - Pergantian hari otomatis (*rollover*): item selesai diarsipkan ke `archived_history.json`, item belum selesai otomatis dibawa ke hari berikutnya (*carry-over*)
  - Toggle status selesai/belum selesai via checkbox interaktif
  - Hapus item tugas dengan ikon '×'
  - Input field tugas custom berbasis software canvas:
    - Cursor caret berkedip (500ms timer)
    - Mengetik teks via keyboard `WM_CHAR` (dukungan Unicode)
    - Hapus huruf dengan `Backspace`
    - Submit item tugas baru dengan tombol `Enter`
    - Paste teks dari Windows Clipboard via `Ctrl+V` (`user32.dll` `GetClipboardData`)
    - `Escape` untuk melepas fokus input box atau menutup pill
- **Jadwal Sholat**:
  - Panel ke-3 berisi hitung mundur sholat berikutnya dan daftar Subuh sampai Isya
  - Pill collapsed menampilkan `Maghrib - 17:49 (00:35)`; menit berkedip merah saat <= 15 menit
  - Banner pengingat 15 menit sebelum waktu sholat; banner + bunyi sistem saat waktu tiba
  - Data dari `api.myquran.com`, di-cache di `%APPDATA%\IslandWin\prayer.json`
- **Tab Switcher** → Tombol switch interaktif "Music", "To-Do", dan "Sholat" di bar kanan atas
- **Escape** → Unpin & collapse pill kembali
- **Hotkey `Ctrl+Alt+I`** → Toggle expand / collapse overlay secara global
- **System Tray Icon** → Klik kanan untuk menu:
  - "Tampilkan / Sembunyikan (Ctrl+Alt+I)"
  - "Pilih Folder Musik..."
  - "Jalankan Saat Windows Startup" (Registry `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` toggle dengan checkmark)
  - "Keluar"
- **Fullscreen Auto-Hide** → Otomatis sembunyi saat game / video fullscreen
- **Single Instance** → Named Mutex guard ("IslandWin_SingleInstance_Mutex")
- **Resource Limits & Memory Tuning (M4)**:
  - `debug.SetGCPercent(50)` dan `debug.SetMemoryLimit(64 MB)`
  - Tidak ada CPU render loop saat idle (event-driven timer 0% CPU)

## Hasil Pengukuran Performa Nyata (Final M4)

| Metrik | Hasil Nyata (Windows 11 x64) | Target PRD | Status |
|--------|------------------------------|------------|--------|
| RAM idle (working set) | **23.63 MB** | < 30 MB | ✅ Tercapai |
| RAM saat putar musik | **< 35 MB** | < 60 MB | ✅ Tercapai |
| Binary size (release `-s -w -H=windowsgui`) | **3.40 MB** | < 15 MB | ✅ Jauh melampaui target |
| Binary size (debug) | **4.79 MB** | < 50 MB | ✅ Tercapai |
| CPU idle | **0%** | 0% | ✅ Tercapai |
| CGO Dependency | **0 (Pure Go)** | 0 (CGO_ENABLED=0) | ✅ Tercapai |

> RAM diukur via `GetProcessMemoryInfo` (PSAPI) pada proses Windows aktif.

## Struktur Kode

```
cmd/islandwin/main.go       entry point, event loop, subsystem wiring
internal/
  todo/
    store.go                Daily task CRUD, rollover, carry-over, atomic JSON save
    store_test.go           Unit tests for task rollover and persistence
  audio/
    decoder.go              MP3 & WAV stream decoders
    library.go              Background recursive folder scanner & JSON cache
    metadata.go             ID3 / filename tag parser
    player.go               Oto WASAPI player, volume scaling, queue logic
  config/config.go          JSON settings (%APPDATA%\IslandWin\config.json)
  prayer/prayer.go          Prayer schedule API client, cache, countdown helpers
  ui/state.go               State machine (Collapsed, ExpandedMusic, ExpandedTodo, ExpandedPrayer)
  win/
    window.go               Win32 layered window, DPI v2, hit-test, events
    tray.go                 Native Win32 Shell_NotifyIcon & context menu
    dialog.go               Native Windows SHBrowseForFolderW folder picker
    clipboard.go            Native Windows Unicode clipboard reader (Ctrl+V)
    autostart.go            Windows registry autostart manager (HKCU Run key)
    single_instance.go      Named mutex & global hotkeys (Ctrl+Alt+I & Media Keys)
    fullscreen.go           SHQueryUserNotificationState detection
  render/
    canvas.go               Software renderer (fogleman/gg), BGRA conversion
    music_ui.go             Music player UI layout & hit-test bounds
    todo_ui.go              To-Do list UI layout, checkboxes, input field & hit-test bounds
    prayer_ui.go            Prayer panel, collapsed pill label, alert banner
  anim/spring.go            Spring animator (pure Go, critically-damped)
scripts/
  build.ps1                 PowerShell build helper (release, debug, test)
```
