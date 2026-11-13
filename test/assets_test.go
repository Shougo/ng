package test

import (
	"bytes"
	"os"
	"testing"
)

// The window icon embedded in ng must be the one installed for launchers.
func TestWindowIconMatchesAssets(t *testing.T) {
	embedded, err := os.ReadFile("../internal/gui/icon.png")
	if err != nil {
		t.Fatal(err)
	}
	installed, err := os.ReadFile("../assets/icons/hicolor/128x128/apps/ng.png")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(embedded, installed) {
		t.Error("internal/gui/icon.png differs from the 128x128 icon; see README (The icon)")
	}
}
