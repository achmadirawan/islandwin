package win

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	procSHQueryUserNotificationState = shell32.NewProc("SHQueryUserNotificationState")
	procGetForegroundWindow          = user32.NewProc("GetForegroundWindow")
	procGetWindowRect                = user32.NewProc("GetWindowRect")
	procGetShellWindow               = user32.NewProc("GetShellWindow")
	procGetDesktopWindow             = user32.NewProc("GetDesktopWindow")
)

type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

// QUERY_USER_NOTIFICATION_STATE values
const (
	QUNS_NOT_PRESENT               = 1
	QUNS_BUSY                      = 2
	QUNS_RUNNING_D3D_FULL_SCREEN   = 3
	QUNS_PRESENTATION_MODE         = 4
	QUNS_ACCEPTS_NOTIFICATIONS     = 5
	QUNS_QUIET_TIME                = 6
	QUNS_APP                       = 7
)

// IsFullscreenAppActive returns true if the current active application is in fullscreen mode.
// Uses SHQueryUserNotificationState first, with a fallback checking foreground window rect.
func IsFullscreenAppActive() bool {
	var state uint32
	ret, _, _ := procSHQueryUserNotificationState.Call(uintptr(unsafe.Pointer(&state)))
	if ret == 0 {
		if state == QUNS_RUNNING_D3D_FULL_SCREEN || state == QUNS_PRESENTATION_MODE || state == QUNS_BUSY {
			return true
		}
	}

	// Fallback: Check if foreground window bounds match or exceed screen size.
	fgHwnd, _, _ := procGetForegroundWindow.Call()
	if fgHwnd == 0 {
		return false
	}

	// Ignore desktop or shell window (desktop is always fullscreen).
	shellHwnd, _, _ := procGetShellWindow.Call()
	desktopHwnd, _, _ := procGetDesktopWindow.Call()
	if fgHwnd == shellHwnd || fgHwnd == desktopHwnd {
		return false
	}

	var r RECT
	ret, _, _ = procGetWindowRect.Call(fgHwnd, uintptr(unsafe.Pointer(&r)))
	if ret == 0 {
		return false
	}

	sw, sh := ScreenSize()
	// If window covers full screen (e.g. 0,0 to sw,sh)
	if r.Left <= 0 && r.Top <= 0 && r.Right >= int32(sw) && r.Bottom >= int32(sh) {
		return true
	}

	return false
}
