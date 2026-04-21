package media

import (
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type Protocol int

const (
	ProtocolNone Protocol = iota
	ProtocolKitty
	ProtocolITerm2
	ProtocolSixel
)

func DetectProtocol() Protocol {
	termProgram := os.Getenv("TERM_PROGRAM")
	term := os.Getenv("TERM")
	if termProgram == "iTerm.app" || termProgram == "WezTerm" {
		return ProtocolITerm2
	}
	if termProgram == "kitty" || strings.Contains(term, "kitty") {
		return ProtocolKitty
	}
	if supportsSixel(term) {
		return ProtocolSixel
	}
	return ProtocolNone
}

func RenderFile(path string, maxCols int) (string, int, error) {
	protocol := DetectProtocol()
	if protocol == ProtocolNone || protocol == ProtocolSixel {
		return "", 0, nil
	}
	if maxCols < 8 {
		maxCols = 8
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, fmt.Errorf("read image: %w", err)
	}
	rows := estimateImageRows(path, maxCols)
	seq := renderBytes(data, maxCols, protocol)
	if seq == "" {
		return "", 0, nil
	}
	seq = wrapPassthrough(seq)
	lines := make([]string, rows)
	lines[0] = seq
	return strings.Join(lines, "\n"), rows, nil
}

func renderBytes(data []byte, maxCols int, protocol Protocol) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	switch protocol {
	case ProtocolITerm2:
		return ansi.ITerm2(fmt.Sprintf("File=inline=1;width=%d;preserveAspectRatio=1:%s", maxCols, encoded))
	case ProtocolKitty:
		return renderKitty(encoded, maxCols)
	default:
		return ""
	}
}

func renderKitty(encoded string, maxCols int) string {
	const chunkSize = 4096
	var b strings.Builder
	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]
		first := i == 0
		last := end == len(encoded)
		opts := make([]string, 0, 4)
		if first {
			opts = append(opts, "a=T", "f=100", fmt.Sprintf("c=%d", maxCols))
		}
		switch {
		case first && !last:
			opts = append(opts, "m=1")
		case !first && !last:
			opts = append(opts, "m=1")
		case !first && last:
			opts = append(opts, "m=0")
		}
		b.WriteString(ansi.KittyGraphics([]byte(chunk), opts...))
	}
	return b.String()
}

func wrapPassthrough(seq string) string {
	if os.Getenv("TMUX") != "" {
		return ansi.TmuxPassthrough(seq)
	}
	term := os.Getenv("TERM")
	if os.Getenv("STY") != "" || strings.HasPrefix(term, "screen") {
		return ansi.ScreenPassthrough(seq, 760)
	}
	return seq
}

func supportsSixel(term string) bool {
	for _, marker := range []string{"sixel", "mlterm", "yaft"} {
		if strings.Contains(term, marker) {
			return true
		}
	}
	return false
}

func estimateImageRows(path string, maxCols int) int {
	const (
		minRows = 4
		maxRows = 18
	)
	file, err := os.Open(path)
	if err != nil {
		return min(maxCols/2, maxRows)
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return min(max(maxCols/2, minRows), maxRows)
	}
	rows := int(math.Round((float64(config.Height) / float64(config.Width)) * float64(maxCols) * 0.5))
	if rows < minRows {
		return minRows
	}
	if rows > maxRows {
		return maxRows
	}
	return rows
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
