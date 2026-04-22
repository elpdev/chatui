package chat

import tea "github.com/charmbracelet/bubbletea"

// paletteViewID names a hosted interactive sub-screen that lives inside the
// command palette's frame. Adding a new view is a two-step process: declare an
// id here, and register a resolver in commandPaletteDeps.resolveView so the
// palette can dispatch Update/Body calls to the correct backing struct.
type paletteViewID int

const (
	paletteViewNone paletteViewID = iota
	paletteViewHelp
	paletteViewPeerDetail
	paletteViewContactVerify
	paletteViewContactRequestSend
	paletteViewAddRelay
	paletteViewContactRequests
	paletteViewAddContact
)

// paletteView is implemented by every detail modal that renders inside the
// palette frame. The palette owns the outer chrome (breadcrumb title, subtitle,
// footer) and delegates the body region plus input handling to the active
// view. Close must be idempotent — the palette may invoke it during back-nav
// even if Open was never called (e.g., after a transient open that never
// rendered).
type paletteView interface {
	Open(ctx viewOpenCtx) tea.Cmd
	Close()
	Update(msg tea.Msg) (handled bool, cmd tea.Cmd)
	Body(width int) string
	Subtitle() string
	Footer() string
}

// viewOpenCtx carries the Model-owned values a view may need at open time.
// Kept intentionally narrow so the palette stays decoupled from the concrete
// Model type. Fields are populated by the Model's onEnterView callback.
type viewOpenCtx struct {
	peerMailbox     string
	peerFingerprint string
}

// viewCompleteMsg is dispatched by a view when it has finished successfully
// (e.g., a contact was imported). The Model responds by closing the palette
// entirely and surfacing any toast the view wants to display.
type viewCompleteMsg struct {
	id    paletteViewID
	toast string
}

// enterViewCmd returns a Bubble Tea command the palette issues when activating
// a view node. The Model intercepts this message and invokes the view's Open.
func enterViewCmd(id paletteViewID) tea.Cmd {
	return func() tea.Msg { return enterViewMsg{id: id} }
}

type enterViewMsg struct {
	id paletteViewID
}
