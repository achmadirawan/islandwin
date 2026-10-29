package win

import (
	"os"
	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	appName    = "IslandWin"
)

// IsAutostartEnabled checks if IslandWin is registered to run at startup.
func IsAutostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetStringValue(appName)
	return err == nil && val != ""
}

// SetAutostart enables or disables launch on Windows startup.
func SetAutostart(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if enable {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		return k.SetStringValue(appName, `"`+exePath+`"`)
	}

	err = k.DeleteValue(appName)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}
