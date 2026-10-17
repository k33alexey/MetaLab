// Package brand holds the icon of MetaLab - the monogram ML - in one place:
// the application shows it on its tab, in its manifest and on a phone's
// home screen, Studio on its tab.
package brand

import "embed"

//go:embed icon.png icon-192.png icon-512.png
var files embed.FS

// Icon returns the icon of one of its sizes by its file name - icon.png and
// icon-512.png of 512 pixels, icon-192.png of 192 - and false for any other.
func Icon(name string) ([]byte, bool) {
	switch name {
	case "icon.png", "icon-192.png", "icon-512.png":
		content, err := files.ReadFile(name)
		return content, err == nil
	}
	return nil, false
}
