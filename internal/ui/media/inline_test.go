package media

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectProtocol(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	t.Setenv("TERM", "xterm-256color")
	if got := DetectProtocol(); got != ProtocolITerm2 {
		t.Fatalf("expected iTerm2 protocol, got %v", got)
	}

	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERM", "xterm-kitty")
	if got := DetectProtocol(); got != ProtocolKitty {
		t.Fatalf("expected kitty protocol, got %v", got)
	}
}

func TestRenderFileITerm2(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "WezTerm")
	t.Setenv("TERM", "wezterm")
	path := writeTinyPNG(t)
	rendered, rows, err := RenderFile(path, 20)
	if err != nil {
		t.Fatalf("render file: %v", err)
	}
	if rows < 1 {
		t.Fatalf("expected rows > 0, got %d", rows)
	}
	if !strings.Contains(rendered, "]1337;") || !strings.Contains(rendered, "inline=1") {
		t.Fatalf("expected OSC 1337 image sequence, got %q", rendered)
	}
}

func TestRenderFileKittyInsideTmux(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERM", "xterm-kitty")
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1,0")
	path := writeTinyPNG(t)
	rendered, _, err := RenderFile(path, 24)
	if err != nil {
		t.Fatalf("render file: %v", err)
	}
	if !strings.Contains(rendered, "\x1bPtmux;") {
		t.Fatalf("expected tmux passthrough, got %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b_G") {
		t.Fatalf("expected kitty graphics payload, got %q", rendered)
	}
	if !strings.Contains(rendered, "z=-1") {
		t.Fatalf("expected kitty placement to render below UI, got %q", rendered)
	}
}

func writeTinyPNG(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tiny.png")
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	img.Set(1, 0, color.NRGBA{G: 255, A: 255})
	img.Set(0, 1, color.NRGBA{B: 255, A: 255})
	img.Set(1, 1, color.NRGBA{R: 255, G: 255, A: 255})
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png fixture: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png fixture: %v", err)
	}
	return path
}
