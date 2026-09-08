package ui

import (
	"os"
	"os/exec"
	"runtime"
)

// Chime plays a short, best-effort notification sound. It never blocks the
// caller and silently does nothing when no sound mechanism is available.
func Chime() {
	go func() {
		switch runtime.GOOS {
		case "linux":
			if startChime("canberra-gtk-play", "-i", "message-new-instant") {
				return
			}
			for _, snd := range []string{
				"/usr/share/sounds/freedesktop/stereo/message.oga",
				"/usr/share/sounds/freedesktop/stereo/bell.oga",
				"/usr/share/sounds/freedesktop/stereo/complete.oga",
			} {
				if _, err := os.Stat(snd); err != nil {
					continue
				}
				for _, player := range []string{"paplay", "pw-play"} {
					if startChime(player, snd) {
						return
					}
				}
			}
			fallbackBell()
		case "windows":
			startChime("powershell", "-NoProfile", "-Command", "[console]::beep(880,200)")
		case "darwin":
			startChime("afplay", "/System/Library/Sounds/Glass.aiff")
		default:
			fallbackBell()
		}
	}()
}

func startChime(name string, args ...string) bool {
	path, err := exec.LookPath(name)
	if err != nil {
		return false
	}
	cmd := exec.Command(path, args...)
	if cmd.Start() != nil {
		return false
	}
	go cmd.Wait()
	return true
}

func fallbackBell() {
	if f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0); err == nil {
		_, _ = f.WriteString("\a")
		_ = f.Close()
	}
}
