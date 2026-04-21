package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/elpdev/pando/internal/ui/chat"
	"github.com/elpdev/pando/internal/ui/style"
)

type App struct {
	chat   *chat.Model
	ready  bool
	width  int
	height int
}

func New(chatModel *chat.Model) *App {
	return &App{chat: chatModel}
}

func (a *App) Init() tea.Cmd {
	return a.chat.Init()
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		a.chat.SetSize(msg.Width-2, msg.Height-headerRows(msg.Width, msg.Height)-1)
		return a, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			_ = a.chat.Close()
			return a, tea.Quit
		}
	}

	_, cmd := a.chat.Update(msg)
	return a, cmd
}

func (a *App) View() string {
	if !a.ready {
		return "loading..."
	}
	return strings.Join([]string{a.renderHeader(), a.chat.View()}, "\n")
}

// Block-letter wordmark rendered in the branded banner. Each row is 20
// columns wide; the banner decorator pads left/right with diagonal slashes
// to fill the terminal width.
var bannerLogo = [3]string{
	"█▀▄ ▄▀█ █▄ █ █▀▄ █▀█",
	"█▀  █▀█ █ ▀█ █ █ █ █",
	"▀   ▀ ▀ ▀  ▀ ▀▀  ▀▀▀",
}

const (
	bannerLogoWidth = 20
	bannerLeadSlash = 4
	bannerMinWidth  = 48 // below this, collapse to the single-line meta row
	bannerMinHeight = 20 // below this, give message history the real estate
)

// headerRows reports how many terminal rows the header occupies given the
// current window size. The chat view uses this to size its message area.
func headerRows(width, height int) int {
	if width < bannerMinWidth || height < bannerMinHeight {
		return 1
	}
	return 4
}

// renderHeader is the branded top strip. On a roomy terminal it renders a
// three-row PANDO block with slash decoration, then a meta line beneath it
// (identity, peer, connection pill, fingerprint). On narrow or short
// terminals it collapses to a single utilitarian row.
//
// All ephemeral feedback continues to live in the chat toast slot, not here.
func (a *App) renderHeader() string {
	if a.width < bannerMinWidth || a.height < bannerMinHeight {
		return a.renderMetaLine()
	}
	lead := style.Faint.Render(strings.Repeat("╱", bannerLeadSlash))
	trailWidth := max(0, a.width-bannerLeadSlash-1-bannerLogoWidth-1)
	trail := style.Faint.Render(strings.Repeat("╱", trailWidth))
	logoStyle := style.StatusInfo.Bold(true)
	rows := make([]string, 0, 4)
	for _, line := range bannerLogo {
		rows = append(rows, lead+" "+logoStyle.Render(line)+" "+trail)
	}
	meta := a.renderMetaLine()
	if meta != "" {
		rows = append(rows, strings.Repeat(" ", bannerLeadSlash+1)+meta)
	}
	return strings.Join(rows, "\n")
}

// renderMetaLine is the single-row status strip: identity, peer arrow +
// name (accent-colored), connection pill, and short fingerprint + verify
// mark. Clipped to terminal width so the pill never wraps.
func (a *App) renderMetaLine() string {
	identity := style.Muted.Render(a.chat.Mailbox())

	peerSeg := ""
	if peer := a.chat.RecipientMailbox(); peer != "" {
		arrow := style.Muted.Render("›")
		peerStyle := style.PeerAccentStyle(a.chat.PeerFingerprint()).Bold(true)
		peerSeg = "  " + arrow + "  " + peerStyle.Render(peer)
	}

	pill := renderConnectionPill(a.chat.ConnectionState(), a.chat.ReconnectDelay(), a.chat.Status())

	fpSeg := ""
	if fp := a.chat.PeerFingerprint(); fp != "" && a.chat.RecipientMailbox() != "" {
		mark, markStyle := style.GlyphUnverified, style.UnverifiedWarn
		if a.chat.PeerVerified() {
			mark, markStyle = style.GlyphVerified, style.VerifiedOk
		}
		fpSeg = style.Muted.Render(style.FormatFingerprintShort(fp)) + " " + markStyle.Render(mark)
	}

	segs := []string{identity + peerSeg, pill}
	if fpSeg != "" {
		segs = append(segs, fpSeg)
	}
	row := strings.Join(segs, "    ")
	return lipgloss.NewStyle().MaxWidth(a.width).Render(row)
}

func renderConnectionPill(state chat.ConnState, delay time.Duration, detail string) string {
	switch state {
	case chat.ConnConnected:
		return style.StatusOk.Render(style.GlyphConnected) + " " + style.Muted.Render("connected")
	case chat.ConnConnecting:
		return style.StatusWarn.Render(style.GlyphReconnecting) + " " + style.Muted.Render("connecting")
	case chat.ConnReconnecting:
		txt := "reconnecting"
		if delay > 0 {
			txt = fmt.Sprintf("reconnecting in %s", delay)
		}
		return style.StatusWarn.Render(style.GlyphReconnecting) + " " + style.Muted.Render(txt)
	case chat.ConnDisconnected:
		txt := "offline"
		if detail != "" {
			txt = detail
		}
		return style.StatusBad.Render(style.GlyphOffline) + " " + style.Muted.Render(txt)
	case chat.ConnAuthFailed:
		txt := "auth failed"
		if detail != "" {
			txt = detail
		}
		return style.StatusBad.Render(style.GlyphAuthFailed) + " " + style.Muted.Render(txt)
	default:
		return ""
	}
}
