package media

import (
	"encoding/base64"
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
}

func writeTinyPNG(t *testing.T) string {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a6nQAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode png fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "tiny.png")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write png fixture: %v", err)
	}
	return path
}
