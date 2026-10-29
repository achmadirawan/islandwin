package win

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procSHBrowseForFolderW   = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListW = shell32.NewProc("SHGetPathFromIDListW")
	procCoTaskMemFree         = ole32.NewProc("CoTaskMemFree")
	ole32                    = windows.NewLazySystemDLL("ole32.dll")
)

type BROWSEINFOW struct {
	HwndOwner      uintptr
	PidlRoot       uintptr
	PszDisplayName *uint16
	LpszTitle      *uint16
	UlFlags        uint32
	Lpfn           uintptr
	LParam         uintptr
	ICount         int32
}

const (
	BIF_RETURNONLYFSDIRS = 0x00000001
	BIF_NEWDIALOGSTYLE   = 0x00000040
)

// SelectFolderDialog prompts user with the native Windows folder picker dialog.
// Returns the selected directory path, or empty string if cancelled.
func SelectFolderDialog(ownerHwnd uintptr, title string) (string, error) {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return "", err
	}

	var bi BROWSEINFOW
	bi.HwndOwner = ownerHwnd
	bi.LpszTitle = titlePtr
	bi.UlFlags = BIF_RETURNONLYFSDIRS | BIF_NEWDIALOGSTYLE

	pidl, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return "", nil // User cancelled
	}
	defer procCoTaskMemFree.Call(pidl)

	var pathBuf [syscall.MAX_PATH]uint16
	r, _, _ := procSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&pathBuf[0])))
	if r == 0 {
		return "", nil
	}

	return syscall.UTF16ToString(pathBuf[:]), nil
}
