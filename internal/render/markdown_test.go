package render

import (
	"testing"
	"time"
)

func TestIsDarkFromCOLORFGBG(t *testing.T) {
	tests := []struct {
		name  string
		value string
		dark  bool
	}{
		{"black bg", "15;0", true},
		{"dark red bg", "7;1", true},
		{"dark blue bg", "7;4", true},
		{"dark cyan bg", "0;6", true},
		{"white bg", "0;7", false},
		{"bright black bg", "7;8", false},
		{"bright white bg", "0;15", false},
		{"three-part dark", "0;7;0", true},
		{"three-part light", "0;7;15", false},
		{"empty string", "", true},
		{"no semicolon", "0", true},
		{"non-numeric bg", "x;y", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDarkFromCOLORFGBG(tt.value)
			if got != tt.dark {
				t.Errorf("isDarkFromCOLORFGBG(%q) = %v, want %v", tt.value, got, tt.dark)
			}
		})
	}
}

func TestResolveGlamourStyleNoHang(t *testing.T) {
	// Verify resolveGlamourStyle completes quickly (no OSC terminal queries).
	done := make(chan struct{})
	go func() {
		_ = resolveGlamourStyle()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("resolveGlamourStyle() took >2s — likely sending OSC queries to the terminal")
	}
}
