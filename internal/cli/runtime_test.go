package cli

import (
	"os"
	"testing"
)

func TestSurfaceSelectionFailsClosed(t *testing.T) {
	tests := []struct {
		name, format                         string
		inTTY, outTTY, nonInteractive, human bool
	}{
		{"human", "text", true, true, false, true},
		{"json", "json", true, true, false, false},
		{"piped input", "text", false, true, false, false},
		{"redirected output", "text", true, false, false, false},
		{"explicit noninteractive", "text", true, true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectSurface(tt.format, tt.inTTY, tt.outTTY, tt.nonInteractive); (got == surfaceHuman) != tt.human {
				t.Fatalf("surface=%s human=%t", got, tt.human)
			}
		})
	}
}

func TestDevNullIsNotATerminal(t *testing.T) {
	file, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if isTerminalFile(file) {
		t.Fatal("/dev/null classified as terminal")
	}
}
