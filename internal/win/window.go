// Package win wraps the Win32 syscalls needed for layered windows,
// message pump, hit testing, DPI awareness, hotkeys, and tray icon.
package win

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")
	kernel = windows.NewLazySystemDLL("kernel32.dll")
	psapi  = windows.NewLazySystemDLL("psapi.dll")

	procRegisterClassExW              = user32.NewProc("RegisterClassExW")
	procCreateWindowExW               = user32.NewProc("CreateWindowExW")
	procDefWindowProcW                = user32.NewProc("DefWindowProcW")
	procUpdateLayeredWindow           = user32.NewProc("UpdateLayeredWindow")
	procGetDC                         = user32.NewProc("GetDC")
	procReleaseDC                     = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC            = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC                      = gdi32.NewProc("DeleteDC")
	procCreateDIBSection              = gdi32.NewProc("CreateDIBSection")
	procSelectObject                  = gdi32.NewProc("SelectObject")
	procDeleteObject                  = gdi32.NewProc("DeleteObject")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	procTrackMouseEvent               = user32.NewProc("TrackMouseEvent")
	procSetTimer                      = user32.NewProc("SetTimer")
	procKillTimer                     = user32.NewProc("KillTimer")
	procPostQuitMessage               = user32.NewProc("PostQuitMessage")
	procGetMessageW                   = user32.NewProc("GetMessageW")
	procTranslateMessage              = user32.NewProc("TranslateMessage")
	procDispatchMessageW              = user32.NewProc("DispatchMessageW")
	procShowWindow                    = user32.NewProc("ShowWindow")
	procSetWindowPos                  = user32.NewProc("SetWindowPos")
	procGetProcessMemoryInfo          = psapi.NewProc("GetProcessMemoryInfo")
	procGetCurrentProcess             = kernel.NewProc("GetCurrentProcess")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	procGetKeyState                   = user32.NewProc("GetKeyState")
	procScreenToClient                = user32.NewProc("ScreenToClient")
	procLoadCursorW                   = user32.NewProc("LoadCursorW")
	procSetFocus                      = user32.NewProc("SetFocus")
)

// Win32 constants
const (
	WS_POPUP         = 0x80000000
	WS_EX_LAYERED    = 0x00080000
	WS_EX_TOPMOST    = 0x00000008
	WS_EX_TOOLWINDOW = 0x00000080
	WS_EX_NOACTIVATE = 0x08000000

	SW_SHOW = 5
	SW_HIDE = 0

	SWP_NOSIZE       = 0x0001
	SWP_NOMOVE       = 0x0002
	SWP_NOZORDER     = 0x0004
	SWP_NOACTIVATE   = 0x0010
	SWP_FRAMECHANGED = 0x0020

	ULW_ALPHA    = 0x00000002
	AC_SRC_OVER  = 0x00
	AC_SRC_ALPHA = 0x01

	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	IDC_ARROW = 32512

	WM_DESTROY       = 0x0002
	WM_KEYDOWN       = 0x0100
	WM_CHAR          = 0x0102
	WM_MOUSEMOVE     = 0x0200
	WM_LBUTTONDOWN   = 0x0201
	WM_LBUTTONUP     = 0x0202
	WM_RBUTTONUP     = 0x0205
	WM_MOUSELEAVE    = 0x02A3
	WM_NCHITTEST     = 0x0084
	WM_TIMER         = 0x0113
	WM_DISPLAYCHANGE = 0x007E

	VK_BACK    = 0x08
	VK_RETURN  = 0x0D
	VK_ESCAPE  = 0x1B
	VK_CONTROL = 0x11
	VK_HOME    = 0x24
	VK_END     = 0x23
	VK_LEFT    = 0x25
	VK_RIGHT   = 0x27
	VK_DELETE  = 0x2E

	HTCLIENT = 1

	TME_LEAVE = 0x00000002

	// DPI Awareness context values
	DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = ^uintptr(3) // -4
)

// Win32 structures
type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

type POINT struct {
	X int32
	Y int32
}

type SIZE struct {
	CX int32
	CY int32
}

type BLENDFUNCTION struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type TRACKMOUSEEVENT struct {
	CbSize      uint32
	DwFlags     uint32
	HwndTrack   windows.Handle
	DwHoverTime uint32
}

type MSG struct {
	Hwnd     windows.Handle
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}

type PROCESS_MEMORY_COUNTERS struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

// Window wraps an HWND and layered-window drawing state.
type Window struct {
	Hwnd          windows.Handle
	Width         int
	Height        int
	PillW         float64 // Current pill width for hit testing
	PillH         float64 // Current pill height for hit testing
	Visible       bool
	Scale         float64 // DPI scale (1.0 = 96 DPI / 100%)

	// Callbacks
	OnMouseEnter       func()
	OnMouseLeave       func()
	OnClick            func(x, y float64)
	OnChar             func(r rune)
	OnKeyDown          func(vk uint32, ctrl bool)
	OnEscape           func()
	OnTick             func(id uintptr)
	OnHotkey           func()
	OnMediaPlayPause   func()
	OnMediaNext        func()
	OnMediaPrev        func()
	OnDisplayChange    func()
	OnTrayToggle          func()
	OnTrayExit            func()
	OnTraySelectFolder    func()
	OnTrayAutostartToggle func()

	// Prayer city menu: label/choices are set by the app, callbacks run on the UI thread.
	PrayerCityLabel       string
	PrayerCityChoices     []string
	OnTrayPrayerSync      func()
	OnTrayPrayerClipboard func()
	OnTrayPrayerCity      func(name string)

	trackingMouse bool
}

var currentWindow *Window

func unsafePointer[T any](val *T) unsafe.Pointer {
	return unsafe.Pointer(val)
}

// EnablePerMonitorDPIAwareness configures Per-Monitor DPI Awareness v2.
func EnablePerMonitorDPIAwareness() {
	if procSetProcessDpiAwarenessContext.Find() == nil {
		procSetProcessDpiAwarenessContext.Call(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)
	}
}

// ScreenSize returns primary monitor width and height in pixels.
func ScreenSize() (int, int) {
	w, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	h, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
	return int(w), int(h)
}

// GetWorkingSetMB returns the current working set in megabytes.
func GetWorkingSetMB() float64 {
	hProc, _, _ := procGetCurrentProcess.Call()
	var counters PROCESS_MEMORY_COUNTERS
	counters.Cb = uint32(unsafe.Sizeof(counters))
	ret, _, _ := procGetProcessMemoryInfo.Call(
		hProc,
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.Cb),
	)
	if ret == 0 {
		return 0
	}
	return float64(counters.WorkingSetSize) / (1024 * 1024)
}

// wndProc is the Win32 window message procedure.
func wndProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	w := currentWindow
	switch msg {
	case WM_NCHITTEST:
		// Hit-test: only return HTCLIENT if mouse is within the active pill geometry.
		// Otherwise return HTTRANSPARENT so windows beneath get the clicks/hovers.
		if w != nil {
			var pt POINT
			pt.X = int32(int16(lParam & 0xFFFF))
			pt.Y = int32(int16((lParam >> 16) & 0xFFFF))
			procScreenToClient.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&pt)))

			rx := float64(pt.X)
			ry := float64(pt.Y)

			// Pill bounding box within canvas: horizontally centered, anchored at top y=0.
			pillX := (float64(w.Width) - w.PillW) / 2
			pillY := 0.0

			if rx >= pillX && rx <= pillX+w.PillW && ry >= pillY && ry <= pillY+w.PillH {
				return HTCLIENT
			}
			// HTTRANSPARENT = -1 as two's complement uintptr
			return ^uintptr(0)
		}
		return ^uintptr(0)

	case WM_MOUSEMOVE:
		if w != nil && !w.trackingMouse {
			var tme TRACKMOUSEEVENT
			tme.CbSize = uint32(unsafe.Sizeof(tme))
			tme.DwFlags = TME_LEAVE
			tme.HwndTrack = hwnd
			procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
			w.trackingMouse = true
			if w.OnMouseEnter != nil {
				w.OnMouseEnter()
			}
		}
		return 0

	case WM_LBUTTONDOWN:
		if w != nil && w.OnClick != nil {
			clickX := float64(int(int16(lParam & 0xFFFF)))
			clickY := float64(int(int16((lParam >> 16) & 0xFFFF)))
			w.OnClick(clickX, clickY)
		}
		return 0

	case WM_LBUTTONUP:
		return 0

	case WM_KEYDOWN:
		if w != nil {
			if wParam == VK_ESCAPE && w.OnEscape != nil {
				w.OnEscape()
			} else if w.OnKeyDown != nil {
				ctrlPressed, _, _ := procGetKeyState.Call(VK_CONTROL)
				isCtrl := (uint16(ctrlPressed) & 0x8000) != 0
				w.OnKeyDown(uint32(wParam), isCtrl)
			}
		}
		return 0

	case WM_CHAR:
		if w != nil && w.OnChar != nil {
			r := rune(wParam)
			w.OnChar(r)
		}
		return 0

	case WM_HOTKEY:
		if w != nil {
			switch wParam {
			case HOTKEY_ID_TOGGLE:
				if w.OnHotkey != nil {
					w.OnHotkey()
				}
			case HOTKEY_ID_PLAYPAUSE:
				if w.OnMediaPlayPause != nil {
					w.OnMediaPlayPause()
				}
			case HOTKEY_ID_NEXT:
				if w.OnMediaNext != nil {
					w.OnMediaNext()
				}
			case HOTKEY_ID_PREV:
				if w.OnMediaPrev != nil {
					w.OnMediaPrev()
				}
			}
		}
		return 0

	case WM_MOUSELEAVE:
		if w != nil {
			w.trackingMouse = false
			if w.OnMouseLeave != nil {
				w.OnMouseLeave()
			}
		}
		return 0

	case WM_TIMER:
		if w != nil && w.OnTick != nil {
			w.OnTick(wParam)
		}
		return 0

	case WM_DISPLAYCHANGE:
		if w != nil && w.OnDisplayChange != nil {
			w.OnDisplayChange()
		}
		return 0

	case WM_APP_TRAY:
		switch lParam {
		case WM_RBUTTONUP:
			if w != nil {
				w.ShowTrayMenu()
			}
		case WM_LBUTTONUP:
			if w != nil && w.OnTrayToggle != nil {
				w.OnTrayToggle()
			}
		}
		return 0

	case WM_DESTROY:
		if w != nil {
			w.RemoveTrayIcon()
			w.UnregisterToggleHotkey()
		}
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

// Focus activates the window to receive keyboard messages.
func (w *Window) Focus() {
	procSetForegroundWindow.Call(uintptr(w.Hwnd))
	procSetFocus.Call(uintptr(w.Hwnd))
}

// IsForeground reports whether this window is the active foreground window.
func (w *Window) IsForeground() bool {
	fg, _, _ := procGetForegroundWindow.Call()
	return fg == uintptr(w.Hwnd)
}

// IsCursorInsidePill checks if the current mouse cursor is within the pill area.
func (w *Window) IsCursorInsidePill() bool {
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procScreenToClient.Call(uintptr(w.Hwnd), uintptr(unsafe.Pointer(&pt)))
	pillX := int32((float64(w.Width) - w.PillW) / 2)
	return pt.X >= pillX && pt.X <= pillX+int32(w.PillW) && pt.Y >= 0 && pt.Y <= int32(w.PillH)
}

// NewWindow registers the window class, creates a layered window, and positions it top-center.
func NewWindow(w, h int) (*Window, error) {
	runtime.LockOSThread() // Message loop must stay on this OS thread

	EnablePerMonitorDPIAwareness()

	className, _ := windows.UTF16PtrFromString("IslandWinClass")
	hInstance := windows.Handle(0)

	hCursor, _, _ := procLoadCursorW.Call(0, uintptr(IDC_ARROW))

	var wc WNDCLASSEXW
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.LpfnWndProc = windows.NewCallback(wndProc)
	wc.HInstance = hInstance
	wc.HCursor = windows.Handle(hCursor)
	wc.LpszClassName = className

	ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		return nil, fmt.Errorf("RegisterClassExW failed: %v", err)
	}

	sw, _ := ScreenSize()
	posX := (sw - w) / 2
	posY := 0 // Anchored to top screen border

	hwndRaw, _, err := procCreateWindowExW.Call(
		WS_EX_LAYERED|WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE,
		uintptr(unsafe.Pointer(className)),
		0,
		WS_POPUP,
		uintptr(posX), uintptr(posY),
		uintptr(w), uintptr(h),
		0, 0,
		uintptr(hInstance),
		0,
	)
	hwnd := windows.Handle(hwndRaw)
	if hwnd == 0 {
		return nil, fmt.Errorf("CreateWindowExW failed: %v", err)
	}

	// Calculate DPI scale
	scale := 1.0
	if procGetDpiForWindow.Find() == nil {
		dpi, _, _ := procGetDpiForWindow.Call(uintptr(hwnd))
		if dpi > 0 {
			scale = float64(dpi) / 96.0
		}
	}

	win := &Window{
		Hwnd:    hwnd,
		Width:   w,
		Height:  h,
		Visible: true,
		Scale:   scale,
	}
	currentWindow = win

	procShowWindow.Call(uintptr(hwnd), SW_SHOW)
	return win, nil
}

// SetVisible shows or hides the overlay window.
func (w *Window) SetVisible(visible bool) {
	w.Visible = visible
	if visible {
		procShowWindow.Call(uintptr(w.Hwnd), SW_SHOW)
	} else {
		procShowWindow.Call(uintptr(w.Hwnd), SW_HIDE)
	}
}

// Reposition recenters the window at the top of the primary monitor.
func (w *Window) Reposition() {
	sw, _ := ScreenSize()
	posX := (sw - w.Width) / 2
	posY := 0
	procSetWindowPos.Call(
		uintptr(w.Hwnd),
		0,
		uintptr(posX), uintptr(posY),
		uintptr(w.Width), uintptr(w.Height),
		SWP_NOZORDER|SWP_NOACTIVATE,
	)
}

// Present uploads BGRA premultiplied-alpha pixel data to the layered window.
func (w *Window) Present(bgraPixels []byte) error {
	if !w.Visible {
		return nil
	}

	hdcScreen, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, hdcScreen)

	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	defer procDeleteDC.Call(hdcMem)

	var bmi BITMAPINFOHEADER
	bmi.BiSize = uint32(unsafe.Sizeof(bmi))
	bmi.BiWidth = int32(w.Width)
	bmi.BiHeight = -int32(w.Height) // Top-down DIB
	bmi.BiPlanes = 1
	bmi.BiBitCount = 32

	var pBits unsafe.Pointer
	hBitmap, _, _ := procCreateDIBSection.Call(
		hdcMem,
		uintptr(unsafe.Pointer(&bmi)),
		0,
		uintptr(unsafe.Pointer(&pBits)),
		0,
		0,
	)
	if hBitmap == 0 {
		return fmt.Errorf("CreateDIBSection failed")
	}
	defer procDeleteObject.Call(hBitmap)

	// Copy BGRA bytes to the DIB section.
	copy(unsafe.Slice((*byte)(pBits), len(bgraPixels)), bgraPixels)

	hOld, _, _ := procSelectObject.Call(hdcMem, hBitmap)
	defer procSelectObject.Call(hdcMem, hOld)

	sw, _ := ScreenSize()
	ptDst := POINT{X: int32((sw - w.Width) / 2), Y: 0}
	ptSrc := POINT{X: 0, Y: 0}
	size := SIZE{CX: int32(w.Width), CY: int32(w.Height)}
	blend := BLENDFUNCTION{
		BlendOp:             AC_SRC_OVER,
		BlendFlags:          0,
		SourceConstantAlpha: 255,
		AlphaFormat:         AC_SRC_ALPHA,
	}

	ret, _, err := procUpdateLayeredWindow.Call(
		uintptr(w.Hwnd),
		hdcScreen,
		uintptr(unsafe.Pointer(&ptDst)),
		uintptr(unsafe.Pointer(&size)),
		hdcMem,
		uintptr(unsafe.Pointer(&ptSrc)),
		0,
		uintptr(unsafe.Pointer(&blend)),
		ULW_ALPHA,
	)
	if ret == 0 {
		return fmt.Errorf("UpdateLayeredWindow failed: %v", err)
	}
	return nil
}

// StartTimer sets a high-frequency Win32 timer (e.g. 16ms for ~60 FPS).
func (w *Window) StartTimer(id uintptr, intervalMs uint32) {
	procSetTimer.Call(uintptr(w.Hwnd), id, uintptr(intervalMs), 0)
}

// StopTimer stops the Win32 timer.
func (w *Window) StopTimer(id uintptr) {
	procKillTimer.Call(uintptr(w.Hwnd), id)
}

// Close posts WM_DESTROY to quit the application cleanly.
func (w *Window) Close() {
	procPostQuitMessage.Call(0)
}

// RunMessageLoop enters the blocking Win32 message pump.
func (w *Window) RunMessageLoop() {
	var msg MSG
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, 0, 0,
		)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
