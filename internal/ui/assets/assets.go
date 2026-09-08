// Package assets holds files compiled into the sjournal binary.
package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.svg
var iconSVG []byte

// AppIcon is the application icon used for the window, the desktop
// notification icon and the system tray. Fyne rasterizes the SVG as needed.
var AppIcon fyne.Resource = fyne.NewStaticResource("sjournal.svg", iconSVG)

// AppID is the Fyne application ID. It must match the installed .desktop
// file's basename (packaging/io.lottkowitz.sjournal.desktop) so that GNOME
// and other Wayland compositors can resolve notifications back to the app.
const AppID = "io.lottkowitz.sjournal"
