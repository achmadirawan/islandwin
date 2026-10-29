package win

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procAppendMenuW      = user32.NewProc("AppendMenuW")
	procDestroyMenu      = user32.NewProc("DestroyMenu")
	procTrackPopupMenuEx = user32.NewProc("TrackPopupMenuEx")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procLoadIconW        = user32.NewProc("LoadIconW")
)

const (
	NIM_ADD    = 0x00000000
	NIM_MODIFY = 0x00000001
	NIM_DELETE = 0x00000002

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	MF_STRING    = 0x00000000
	MF_CHECKED   = 0x00000008
	MF_UNCHECKED = 0x00000000
	MF_SEPARATOR = 0x00000800
	MF_GRAYED    = 0x00000001
	MF_POPUP     = 0x00000010

	TPM_RIGHTBUTTON = 0x0002
	TPM_RETURNCMD   = 0x0100

	IDI_APPLICATION = 32512

	WM_APP_TRAY = 0x8000 + 100 // Custom window message for tray callbacks

	// Menu command IDs
	IDM_TOGGLE        = 1001
	IDM_SELECT_FOLDER = 1002
	IDM_AUTOSTART     = 1003
	IDM_EXIT          = 1004
	IDM_PRAYER_SYNC   = 1005
	IDM_PRAYER_CLIP   = 1006
	IDM_PRAYER_CITY0  = 2000 // + index into PrayerCityChoices
)

type NOTIFYICONDATAW struct {
	CbSize           uint32
	Hwnd             windows.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            windows.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UTimeoutOrVersion uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     windows.Handle
}

// AddTrayIcon creates the system tray notification icon for the given HWND.
func (w *Window) AddTrayIcon(tip string) error {
	hIcon, _, _ := procLoadIconW.Call(0, uintptr(IDI_APPLICATION))

	var nid NOTIFYICONDATAW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.Hwnd = w.Hwnd
	nid.UID = 1
	nid.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	nid.UCallbackMessage = WM_APP_TRAY
	nid.HIcon = windows.Handle(hIcon)

	tipUTF16, _ := windows.UTF16FromString(tip)
	copy(nid.SzTip[:], tipUTF16)

	ret, _, _ := procShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
	if ret == 0 {
		return windows.GetLastError()
	}
	return nil
}

// RemoveTrayIcon cleans up the system tray notification icon.
func (w *Window) RemoveTrayIcon() {
	var nid NOTIFYICONDATAW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.Hwnd = w.Hwnd
	nid.UID = 1
	procShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
}

// ShowTrayMenu displays a popup menu at the cursor position.
func (w *Window) ShowTrayMenu() {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	titleToggle, _ := windows.UTF16PtrFromString("Tampilkan / Sembunyikan (Ctrl+Alt+I)")
	titleFolder, _ := windows.UTF16PtrFromString("Pilih Folder Musik...")
	titleAutostart, _ := windows.UTF16PtrFromString("Jalankan Saat Windows Startup")
	titleExit, _ := windows.UTF16PtrFromString("Keluar")

	procAppendMenuW.Call(hMenu, MF_STRING, IDM_TOGGLE, uintptr(unsafe.Pointer(titleToggle)))
	procAppendMenuW.Call(hMenu, MF_STRING, IDM_SELECT_FOLDER, uintptr(unsafe.Pointer(titleFolder)))

	autostartFlags := uintptr(MF_STRING)
	if IsAutostartEnabled() {
		autostartFlags |= MF_CHECKED
	}
	procAppendMenuW.Call(hMenu, autostartFlags, IDM_AUTOSTART, uintptr(unsafe.Pointer(titleAutostart)))

	procAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)

	// Prayer city section. Submenu is owned by hMenu and freed with it.
	label, _ := windows.UTF16PtrFromString("Kota sholat: " + w.PrayerCityLabel)
	procAppendMenuW.Call(hMenu, MF_STRING|MF_GRAYED, 0, uintptr(unsafe.Pointer(label)))
	titleSync, _ := windows.UTF16PtrFromString("Sinkron Kota Otomatis (IP)")
	procAppendMenuW.Call(hMenu, MF_STRING, IDM_PRAYER_SYNC, uintptr(unsafe.Pointer(titleSync)))
	titleClip, _ := windows.UTF16PtrFromString("Kota dari Clipboard")
	procAppendMenuW.Call(hMenu, MF_STRING, IDM_PRAYER_CLIP, uintptr(unsafe.Pointer(titleClip)))
	if len(w.PrayerCityChoices) > 0 {
		sub, _, _ := procCreatePopupMenu.Call()
		for i, name := range w.PrayerCityChoices {
			n, _ := windows.UTF16PtrFromString(name)
			procAppendMenuW.Call(sub, MF_STRING, uintptr(IDM_PRAYER_CITY0+i), uintptr(unsafe.Pointer(n)))
		}
		titleList, _ := windows.UTF16PtrFromString("Pilih Kota Manual")
		procAppendMenuW.Call(hMenu, MF_POPUP, sub, uintptr(unsafe.Pointer(titleList)))
	}

	procAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)
	procAppendMenuW.Call(hMenu, MF_STRING, IDM_EXIT, uintptr(unsafe.Pointer(titleExit)))

	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	// SetForegroundWindow is required by Win32 before TrackPopupMenu to dismiss properly.
	procSetForegroundWindow.Call(uintptr(w.Hwnd))

	cmd, _, _ := procTrackPopupMenuEx.Call(
		hMenu,
		TPM_RIGHTBUTTON|TPM_RETURNCMD,
		uintptr(pt.X),
		uintptr(pt.Y),
		uintptr(w.Hwnd),
		0,
	)

	switch cmd {
	case IDM_TOGGLE:
		if w.OnTrayToggle != nil {
			w.OnTrayToggle()
		}
	case IDM_SELECT_FOLDER:
		if w.OnTraySelectFolder != nil {
			w.OnTraySelectFolder()
		}
	case IDM_AUTOSTART:
		if w.OnTrayAutostartToggle != nil {
			w.OnTrayAutostartToggle()
		}
	case IDM_EXIT:
		if w.OnTrayExit != nil {
			w.OnTrayExit()
		}
	case IDM_PRAYER_SYNC:
		if w.OnTrayPrayerSync != nil {
			w.OnTrayPrayerSync()
		}
	case IDM_PRAYER_CLIP:
		if w.OnTrayPrayerClipboard != nil {
			w.OnTrayPrayerClipboard()
		}
	default:
		if i := int(cmd) - IDM_PRAYER_CITY0; i >= 0 && i < len(w.PrayerCityChoices) && w.OnTrayPrayerCity != nil {
			w.OnTrayPrayerCity(w.PrayerCityChoices[i])
		}
	}
}
