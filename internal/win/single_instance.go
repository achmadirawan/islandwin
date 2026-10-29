package win

import (
	"fmt"

	"golang.org/x/sys/windows"
)

var (
	procCreateMutexW = kernel.NewProc("CreateMutexW")
	procReleaseMutex = kernel.NewProc("ReleaseMutex")
	procCloseHandle  = kernel.NewProc("CloseHandle")

	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
)

const (
	ERROR_ALREADY_EXISTS = 183

	// Hotkey modifier flags
	MOD_ALT     = 0x0001
	MOD_CONTROL = 0x0002
	MOD_SHIFT   = 0x0004
	MOD_WIN     = 0x0008

	WM_HOTKEY = 0x0312

	HOTKEY_ID_TOGGLE    = 1
	HOTKEY_ID_PLAYPAUSE = 2
	HOTKEY_ID_NEXT      = 3
	HOTKEY_ID_PREV      = 4

	VK_MEDIA_NEXT_TRACK = 0xB0
	VK_MEDIA_PREV_TRACK = 0xB1
	VK_MEDIA_STOP       = 0xB2
	VK_MEDIA_PLAY_PAUSE = 0xB3
)

type SingleInstance struct {
	handle windows.Handle
}

// TryAcquireSingleInstance creates a named mutex to ensure only one instance runs.
// Returns an error if an instance is already running.
func TryAcquireSingleInstance(name string) (*SingleInstance, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}

	h, _, _ := procCreateMutexW.Call(0, 0, uintptr(unsafePointer(namePtr)))
	if h == 0 {
		return nil, fmt.Errorf("failed to create named mutex: %v", windows.GetLastError())
	}

	if windows.GetLastError() == windows.Errno(ERROR_ALREADY_EXISTS) {
		procCloseHandle.Call(h)
		return nil, fmt.Errorf("instance already running")
	}

	return &SingleInstance{handle: windows.Handle(h)}, nil
}

func (s *SingleInstance) Release() {
	if s.handle != 0 {
		procReleaseMutex.Call(uintptr(s.handle))
		procCloseHandle.Call(uintptr(s.handle))
		s.handle = 0
	}
}

// RegisterToggleHotkey registers Ctrl+Alt+I globally.
func (w *Window) RegisterToggleHotkey() error {
	// 'I' key code is 0x49
	ret, _, err := procRegisterHotKey.Call(
		uintptr(w.Hwnd),
		HOTKEY_ID_TOGGLE,
		MOD_CONTROL|MOD_ALT,
		0x49, // VK 'I'
	)
	if ret == 0 {
		return fmt.Errorf("RegisterHotKey failed: %v", err)
	}
	return nil
}

// RegisterMediaHotkeys registers global hardware media buttons (Play/Pause, Next, Prev).
func (w *Window) RegisterMediaHotkeys() {
	procRegisterHotKey.Call(uintptr(w.Hwnd), HOTKEY_ID_PLAYPAUSE, 0, VK_MEDIA_PLAY_PAUSE)
	procRegisterHotKey.Call(uintptr(w.Hwnd), HOTKEY_ID_NEXT, 0, VK_MEDIA_NEXT_TRACK)
	procRegisterHotKey.Call(uintptr(w.Hwnd), HOTKEY_ID_PREV, 0, VK_MEDIA_PREV_TRACK)
}

// UnregisterToggleHotkey unregisters the hotkey.
func (w *Window) UnregisterToggleHotkey() {
	procUnregisterHotKey.Call(uintptr(w.Hwnd), HOTKEY_ID_TOGGLE)
	procUnregisterHotKey.Call(uintptr(w.Hwnd), HOTKEY_ID_PLAYPAUSE)
	procUnregisterHotKey.Call(uintptr(w.Hwnd), HOTKEY_ID_NEXT)
	procUnregisterHotKey.Call(uintptr(w.Hwnd), HOTKEY_ID_PREV)
}
