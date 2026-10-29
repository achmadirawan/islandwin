package win

// Beep plays the system "asterisk" sound (async, honors user's sound scheme).
func Beep() {
	user32.NewProc("MessageBeep").Call(0x40)
}
