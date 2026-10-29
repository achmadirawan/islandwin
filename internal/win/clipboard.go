package win

import (
	"syscall"
	"unsafe"
)

var (
	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procGlobalLock       = kernel.NewProc("GlobalLock")
	procGlobalUnlock     = kernel.NewProc("GlobalUnlock")
)

const CF_UNICODETEXT = 13

// GetClipboardText retrieves current unicode text from Windows clipboard.
func GetClipboardText(hwnd uintptr) (string, error) {
	r, _, _ := procOpenClipboard.Call(hwnd)
	if r == 0 {
		return "", syscall.GetLastError()
	}
	defer procCloseClipboard.Call()

	hData, _, _ := procGetClipboardData.Call(CF_UNICODETEXT)
	if hData == 0 {
		return "", nil // No text on clipboard
	}

	pData, _, _ := procGlobalLock.Call(hData)
	if pData == 0 {
		return "", nil
	}
	defer procGlobalUnlock.Call(hData)

	return syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(pData))[:]), nil
}
