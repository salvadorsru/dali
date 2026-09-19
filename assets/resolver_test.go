package assets

import "testing"

func TestDir(t *testing.T) {
	if got := Dir("/tmp/design.fig"); got != "/tmp/design.assets" {
		t.Fatalf("Dir = %q", got)
	}
	if got := ImagesDir("/tmp/design.fig"); got != "/tmp/design.assets/images" {
		t.Fatalf("ImagesDir = %q", got)
	}
}

func TestExt(t *testing.T) {
	if got := Ext("image/png"); got != ".png" {
		t.Fatalf("Ext = %q", got)
	}
	if got := Ext("image/jpeg"); got != ".jpg" {
		t.Fatalf("Ext = %q", got)
	}
}
