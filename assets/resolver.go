package assets

import (
	"path/filepath"
	"strings"
)

// Dir returns the asset directory for a fig file: design.fig -> design.assets.
func Dir(figPath string) string {
	base := strings.TrimSuffix(figPath, filepath.Ext(figPath))
	return base + ".assets"
}

func ImagesDir(figPath string) string {
	return filepath.Join(Dir(figPath), "images")
}

// Ext maps an image mime type to a file extension.
func Ext(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".bin"
	}
}
