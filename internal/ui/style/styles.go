// Package style holds the visual tokens for the Pando TUI.
//
// All exported Style values below are populated by Apply (see theme.go) at
// package init and on every subsequent theme swap. Downstream rendering
// code consumes these named tokens (Muted, StatusOk, Modal, ...) rather
// than picking raw lipgloss.Color values, so a palette change never has
// to touch rendering code.
package style

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ----------------------------------------------------------------------------
// Foreground / emphasis tokens.
// ----------------------------------------------------------------------------

var (
	// Muted is secondary copy: hints, timestamps, fingerprints.
	Muted lipgloss.Style
	// Subtle is tertiary copy: meta counters, footnotes.
	Subtle lipgloss.Style
	// Dim is body copy inside a darker container (modals).
	Dim lipgloss.Style
	// Bright is the highest-contrast foreground for headings inside modals.
	Bright lipgloss.Style
	// Faint is barely-visible text for thin dividers and placeholder borders.
	Faint lipgloss.Style

	// Bold and Italic are orthogonal to color and never change with the theme.
	Bold   = lipgloss.NewStyle().Bold(true)
	Italic = lipgloss.NewStyle().Italic(true)

	// ModalTitle is the bold bright heading at the top of every modal.
	ModalTitle lipgloss.Style
)

// ----------------------------------------------------------------------------
// Status tokens — for connection, toasts, badges.
// ----------------------------------------------------------------------------

var (
	StatusOk   lipgloss.Style
	StatusWarn lipgloss.Style
	StatusBad  lipgloss.Style
	StatusInfo lipgloss.Style
)

// ----------------------------------------------------------------------------
// Semantic tokens — meaning-bearing aliases. Prefer these at call sites so
// intent is obvious in the rendering code.
// ----------------------------------------------------------------------------

var (
	VerifiedOk     lipgloss.Style
	UnverifiedWarn lipgloss.Style

	DeliveryPending   lipgloss.Style
	DeliverySent      lipgloss.Style
	DeliveryDelivered lipgloss.Style
	DeliveryFailed    lipgloss.Style

	UnreadBadge lipgloss.Style

	// CursorBlock styles the blinking block cursor in the add-contact editor.
	CursorBlock lipgloss.Style
)

// ----------------------------------------------------------------------------
// Surfaces — backgrounds, selection highlight, borders.
// ----------------------------------------------------------------------------

var (
	Selected  lipgloss.Style
	BgModal   lipgloss.Style
	ActiveRow lipgloss.Style

	// BackdropTint is the raw color used to tint whitespace around modals via
	// lipgloss.WithWhitespaceBackground.
	BackdropTint lipgloss.Color

	// RoomAccent is the signature color for encrypted rooms (fingerprint-less).
	RoomAccent lipgloss.Color

	SidebarBorder        lipgloss.Style
	SidebarBorderFocused lipgloss.Style

	ModalBorder lipgloss.Style
	Modal       lipgloss.Style

	PaletteModal        lipgloss.Style
	PaletteInput        lipgloss.Style
	PaletteItem         lipgloss.Style
	PaletteSelectedItem lipgloss.Style

	InputBorder lipgloss.Style
	InputFrame  = lipgloss.NewStyle()

	PaletteTitle    lipgloss.Style
	PaletteMeta     lipgloss.Style
	PaletteFooter   lipgloss.Style
	PaletteShortcut lipgloss.Style
	PaletteAccent   lipgloss.Style
	PaletteMatch    lipgloss.Style
)

// ----------------------------------------------------------------------------
// Banner — welcome-screen PANDO wordmark.
// ----------------------------------------------------------------------------

var (
	// BannerText renders the PANDO wordmark rows in the welcome-screen banner.
	BannerText lipgloss.Style
	// BannerSlash renders the diagonal-slash decoration bracketing the wordmark.
	BannerSlash lipgloss.Style
)

// ----------------------------------------------------------------------------
// Glyphs — unicode symbols used by rendering code. Exported as constants so
// tests can assert against them without depending on lipgloss styling.
// ----------------------------------------------------------------------------

const (
	GlyphConnected    = "●"
	GlyphReconnecting = "◐"
	GlyphOffline      = "○"
	GlyphAuthFailed   = "⚠"

	GlyphVerified   = "✓"
	GlyphUnverified = "?"

	GlyphDeliveryPending   = "⋯"
	GlyphDeliverySent      = "✓"
	GlyphDeliveryDelivered = "✓✓"
	GlyphDeliveryFailed    = "!"

	GlyphCursorRow    = "▌" // sidebar: keyboard cursor marker
	GlyphActiveChat   = "●" // sidebar: currently open chat marker
	GlyphUnreadDot    = "●" // sidebar: unread-count bullet
	GlyphJumpToLatest = "↓"
	GlyphPrompt       = "›"

	GroupSep = "·" // fingerprint group separator
)

// ----------------------------------------------------------------------------
// Peer accent palette — a stable, small set of colors assigned per-fingerprint
// so the same peer always renders in the same color. Sourced from the active
// theme; the selection is stable across runs because it's hashed, not random.
// ----------------------------------------------------------------------------

// PeerAccent returns a stable color for the given fingerprint. An empty
// fingerprint falls back to the active theme's ok (success) color.
func PeerAccent(fingerprint string) lipgloss.Color {
	if fingerprint == "" {
		return active.Ok
	}
	// Polynomial rolling hash — stable across runs, no crypto dependency.
	var n uint32
	for _, r := range fingerprint {
		n = n*131 + uint32(r)
	}
	palette := active.PeerAccents
	return palette[int(n)%len(palette)]
}

// PeerAccentStyle is PeerAccent wrapped in a lipgloss.Style for direct
// rendering of a mailbox name.
func PeerAccentStyle(fingerprint string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(PeerAccent(fingerprint))
}

// ----------------------------------------------------------------------------
// Fingerprint formatting — shared between TUI and ctlcmd so fingerprints read
// the same everywhere. Input is any hex string; non-hex runes are preserved.
// Empty input returns "".
// ----------------------------------------------------------------------------

// FormatFingerprint renders a fingerprint in 4-rune groups separated by "·".
//
//	FormatFingerprint("abcdef0123456789") == "abcd·ef01·2345·6789"
func FormatFingerprint(fp string) string {
	return formatGroups(fp, 4)
}

// FormatFingerprintShort returns up to the first 8 runes of the fingerprint,
// grouped. Useful for compact header/pill rendering.
//
//	FormatFingerprintShort("abcdef0123456789") == "abcd·ef01"
func FormatFingerprintShort(fp string) string {
	runes := []rune(fp)
	if len(runes) > 8 {
		runes = runes[:8]
	}
	return formatGroups(string(runes), 4)
}

func formatGroups(s string, group int) string {
	if s == "" || group <= 0 {
		return s
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && i%group == 0 {
			b.WriteString(GroupSep)
		}
		b.WriteRune(r)
	}
	return b.String()
}
