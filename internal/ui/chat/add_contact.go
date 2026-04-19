package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/elpdev/pando/internal/identity"
	"github.com/elpdev/pando/internal/rendezvous"
	"github.com/elpdev/pando/internal/ui/style"
)

// addContactMode is the sub-screen inside the add-contact modal. The modal
// opens on the chooser; the user picks one of three onboarding paths with a
// single keystroke, which switches the sub-screen.
type addContactMode int

const (
	addContactModeChooser addContactMode = iota
	addContactModePaste
	addContactModeLookup
	addContactModeInviteChoice
	addContactModeInviteStart
	addContactModeInviteAccept
)

// inviteExchangeTimeout bounds a single rendezvous polling session. Matches
// the CLI default so TUI users aren't surprised by different behaviour.
const inviteExchangeTimeout = 2 * time.Minute

// addContactState is the full state of the add-contact modal. The modal is
// a multi-mode dialog: paste (existing), lookup-by-mailbox, and the two
// sides of the invite-code rendezvous.
type addContactState struct {
	open    bool
	mode    addContactMode
	value   string             // shared input buffer (paste text / mailbox / code)
	code    string             // generated invite code (invite-start mode)
	error   string
	busy    bool               // true while an async op is in flight
	preview *identity.Contact  // paste mode: parsed invite awaiting commit
	cancel  context.CancelFunc // non-nil while an async op is cancellable
}

func (m *Model) openAddContactModal() {
	m.addContact = addContactState{open: true, mode: addContactModeChooser}
	m.input.Blur()
}

func (m *Model) closeAddContactModal(keepStatus bool) {
	if m.addContact.cancel != nil {
		m.addContact.cancel()
	}
	m.addContact = addContactState{}
	if !keepStatus {
		m.pushToast("add contact cancelled", ToastInfo)
	}
	m.input.Focus()
}

// finishAddContact is the shared success path for paste, lookup, and invite
// flows — upsert the new contact into the sidebar, activate the chat, close
// the modal, and post a toast describing how the contact was added.
func (m *Model) finishAddContact(contact *identity.Contact, toastText string) {
	m.upsertContact(contact)
	m.selectContact(contact.AccountID)
	m.activateSelectedContact()
	m.closeAddContactModal(true)
	m.pushToast(toastText, ToastInfo)
}

func (m *Model) handleAddContactKey(msg tea.KeyMsg) (*Model, tea.Cmd) {
	// While an async op is in flight, only Esc is live. Esc cancels the
	// context but keeps busy=true until the result message lands — that way
	// we don't race with the tea.Cmd goroutine.
	if m.addContact.busy {
		if msg.Type == tea.KeyEsc && m.addContact.cancel != nil {
			m.addContact.cancel()
			m.addContact.cancel = nil
		}
		return m, nil
	}

	switch m.addContact.mode {
	case addContactModeChooser:
		return m.handleAddContactChooserKey(msg)
	case addContactModePaste:
		return m.handleAddContactPasteKey(msg)
	case addContactModeLookup:
		return m.handleAddContactLookupKey(msg)
	case addContactModeInviteChoice:
		return m.handleAddContactInviteChoiceKey(msg)
	case addContactModeInviteStart:
		return m.handleAddContactInviteStartKey(msg)
	case addContactModeInviteAccept:
		return m.handleAddContactInviteAcceptKey(msg)
	}
	return m, nil
}

func (m *Model) handleAddContactInviteChoiceKey(msg tea.KeyMsg) (*Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		m.addContact.mode = addContactModeChooser
		m.addContact.error = ""
		return m, nil
	}
	if msg.Type != tea.KeyRunes {
		return m, nil
	}
	switch strings.ToLower(string(msg.Runes)) {
	case "s":
		m.addContact.mode = addContactModeInviteStart
		m.addContact.error = ""
		return m, m.startInviteCmd()
	case "a":
		m.addContact.mode = addContactModeInviteAccept
		m.addContact.value = ""
		m.addContact.error = ""
	}
	return m, nil
}

func (m *Model) handleAddContactChooserKey(msg tea.KeyMsg) (*Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		m.closeAddContactModal(false)
		return m, nil
	}
	if msg.Type != tea.KeyRunes {
		return m, nil
	}
	switch strings.ToLower(string(msg.Runes)) {
	case "p":
		m.addContact.mode = addContactModePaste
		m.addContact.value = ""
		m.addContact.error = ""
		m.addContact.preview = nil
	case "l":
		if !m.relayConfigured() {
			m.addContact.error = "no relay configured"
			return m, nil
		}
		m.addContact.mode = addContactModeLookup
		m.addContact.value = ""
		m.addContact.error = ""
	case "i":
		if !m.relayConfigured() {
			m.addContact.error = "no relay configured"
			return m, nil
		}
		m.addContact.mode = addContactModeInviteChoice
		m.addContact.value = ""
		m.addContact.code = ""
		m.addContact.error = ""
	}
	return m, nil
}

func (m *Model) handleAddContactPasteKey(msg tea.KeyMsg) (*Model, tea.Cmd) {
	if m.addContact.preview != nil {
		switch msg.Type {
		case tea.KeyEsc:
			m.addContact.preview = nil
			m.addContact.error = ""
		case tea.KeyCtrlS:
			m.addContact.error = ""
			m.addContact.busy = true
			return m, m.importPasteCmd(strings.TrimSpace(m.addContact.value))
		}
		return m, nil
	}
	switch msg.Type {
	case tea.KeyEsc:
		m.addContact.mode = addContactModeChooser
		m.addContact.value = ""
		m.addContact.error = ""
		return m, nil
	case tea.KeyCtrlS:
		trimmed := strings.TrimSpace(m.addContact.value)
		if trimmed == "" {
			m.addContact.error = "invite input is empty"
			return m, nil
		}
		contact, err := m.messaging.PreviewContactInviteText(trimmed)
		if err != nil {
			m.addContact.error = err.Error()
			return m, nil
		}
		m.addContact.error = ""
		m.addContact.preview = contact
		return m, nil
	case tea.KeyEnter, tea.KeyCtrlJ:
		m.appendAddContactText("\n")
	case tea.KeyBackspace, tea.KeyCtrlH, tea.KeyDelete:
		m.deleteAddContactRune()
	case tea.KeyCtrlU:
		m.addContact.value = ""
		m.addContact.error = ""
	case tea.KeyRunes:
		m.appendAddContactText(string(msg.Runes))
	}
	return m, nil
}

func (m *Model) handleAddContactLookupKey(msg tea.KeyMsg) (*Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.addContact.mode = addContactModeChooser
		m.addContact.value = ""
		m.addContact.error = ""
		return m, nil
	case tea.KeyEnter, tea.KeyCtrlS:
		mailbox := strings.TrimSpace(m.addContact.value)
		if mailbox == "" {
			m.addContact.error = "mailbox is required"
			return m, nil
		}
		m.addContact.error = ""
		m.addContact.busy = true
		return m, m.lookupContactCmd(mailbox)
	case tea.KeyBackspace, tea.KeyCtrlH, tea.KeyDelete:
		m.deleteAddContactRune()
	case tea.KeyCtrlU:
		m.addContact.value = ""
		m.addContact.error = ""
	case tea.KeyRunes:
		m.appendAddContactText(string(msg.Runes))
	}
	return m, nil
}

func (m *Model) handleAddContactInviteStartKey(msg tea.KeyMsg) (*Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		if m.addContact.cancel != nil {
			m.addContact.cancel()
			m.addContact.cancel = nil
		}
		m.addContact.mode = addContactModeInviteChoice
		m.addContact.value = ""
		m.addContact.code = ""
		m.addContact.error = ""
	}
	return m, nil
}

func (m *Model) handleAddContactInviteAcceptKey(msg tea.KeyMsg) (*Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.addContact.mode = addContactModeInviteChoice
		m.addContact.value = ""
		m.addContact.error = ""
		return m, nil
	case tea.KeyEnter, tea.KeyCtrlS:
		code := strings.TrimSpace(m.addContact.value)
		if code == "" {
			m.addContact.error = "invite code is required"
			return m, nil
		}
		m.addContact.error = ""
		m.addContact.busy = true
		return m, m.acceptInviteCmd(code)
	case tea.KeyBackspace, tea.KeyCtrlH, tea.KeyDelete:
		m.deleteAddContactRune()
	case tea.KeyCtrlU:
		m.addContact.value = ""
		m.addContact.error = ""
	case tea.KeyRunes:
		m.appendAddContactText(string(msg.Runes))
	}
	return m, nil
}

func (m *Model) appendAddContactText(text string) {
	if text == "" || len([]rune(m.addContact.value)) >= addContactLimit {
		return
	}
	remaining := addContactLimit - len([]rune(m.addContact.value))
	runes := []rune(text)
	if len(runes) > remaining {
		runes = runes[:remaining]
	}
	m.addContact.value += string(runes)
	m.addContact.error = ""
}

func (m *Model) deleteAddContactRune() {
	runes := []rune(m.addContact.value)
	if len(runes) == 0 {
		return
	}
	m.addContact.value = string(runes[:len(runes)-1])
	m.addContact.error = ""
}

func (m *Model) relayConfigured() bool {
	return strings.TrimSpace(m.relayURL) != ""
}

// importPasteCmd posts an addContactResultMsg with the imported contact or
// a parse/verify error.
func (m *Model) importPasteCmd(text string) tea.Cmd {
	return func() tea.Msg {
		contact, err := m.messaging.ImportContactInviteText(text, true)
		if err != nil {
			return addContactResultMsg{err: err}
		}
		return addContactResultMsg{contact: contact}
	}
}

// lookupContactCmd looks the mailbox up in the relay directory and saves
// the result with trust source "relay-directory".
func (m *Model) lookupContactCmd(mailbox string) tea.Cmd {
	client, err := m.ensureRelayClient()
	if err != nil {
		return func() tea.Msg { return lookupContactResultMsg{err: err} }
	}
	return func() tea.Msg {
		contact, err := m.messaging.ImportDirectoryContact(client, mailbox)
		return lookupContactResultMsg{contact: contact, err: err}
	}
}

// startInviteCmd generates a code, uploads this device's invite bundle to
// the rendezvous slot, and polls for the peer. Returns a Batch of two
// commands — the first publishes the code to the UI so it renders as soon
// as generation succeeds, the second runs the blocking exchange.
func (m *Model) startInviteCmd() tea.Cmd {
	code, err := rendezvous.GenerateCode()
	if err != nil {
		return func() tea.Msg { return inviteStartedMsg{err: err} }
	}
	m.addContact.code = code
	m.addContact.busy = true
	ctx, cancel := context.WithTimeout(context.Background(), inviteExchangeTimeout)
	m.addContact.cancel = cancel
	return tea.Batch(
		func() tea.Msg { return inviteStartedMsg{code: code} },
		m.runInviteExchangeCmd(ctx, code),
	)
}

// acceptInviteCmd runs the peer side of the invite exchange with a
// user-supplied code.
func (m *Model) acceptInviteCmd(code string) tea.Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), inviteExchangeTimeout)
	m.addContact.cancel = cancel
	return m.runInviteExchangeCmd(ctx, code)
}

func (m *Model) runInviteExchangeCmd(ctx context.Context, code string) tea.Cmd {
	client, err := m.ensureRelayClient()
	if err != nil {
		return func() tea.Msg { return inviteExchangeResultMsg{err: err} }
	}
	id := m.messaging.Identity()
	return func() tea.Msg {
		bundle, err := rendezvous.Exchange(ctx, rendezvous.PollConfig{
			Client:        client,
			Code:          code,
			Self:          id.InviteBundle(),
			SelfAccountID: id.AccountID,
		})
		if err != nil {
			return inviteExchangeResultMsg{err: err, cancelled: ctx.Err() == context.Canceled}
		}
		contact, err := m.messaging.ImportInviteCodeContact(*bundle)
		return inviteExchangeResultMsg{contact: contact, err: err}
	}
}

func (m *Model) renderAddContactModal(base string) string {
	modalWidth := min(max(58, m.width*2/3), max(40, m.width-6))
	modalHeight := min(max(15, m.height*2/3), max(12, m.height-4))
	if modalWidth <= 0 || modalHeight <= 0 {
		return base
	}
	bodyWidth := max(24, modalWidth-6)

	title := style.Bright.Bold(true).Render("Add Contact")
	parts := []string{title}

	switch m.addContact.mode {
	case addContactModeChooser:
		parts = append(parts, m.renderAddContactChooser(bodyWidth))
	case addContactModePaste:
		parts = append(parts, m.renderAddContactPaste(bodyWidth, modalHeight))
	case addContactModeLookup:
		parts = append(parts, m.renderAddContactLookup(bodyWidth))
	case addContactModeInviteChoice:
		parts = append(parts, m.renderAddContactInviteChoice(bodyWidth))
	case addContactModeInviteStart:
		parts = append(parts, m.renderAddContactInviteStart(bodyWidth))
	case addContactModeInviteAccept:
		parts = append(parts, m.renderAddContactInviteAccept(bodyWidth))
	}

	if m.addContact.error != "" {
		parts = append(parts, style.StatusBad.Width(bodyWidth).Render(m.addContact.error))
	}
	parts = append(parts, style.Subtle.Render(m.addContactFooter()))

	modal := style.Modal.Width(modalWidth).Padding(1, 2).Render(strings.Join(parts, "\n\n"))
	background := style.Faint.Render(base)
	return strings.Join([]string{background, lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)}, "\n")
}

func (m *Model) renderAddContactChooser(width int) string {
	row := func(key, label, hint string, disabled bool) string {
		key = style.Bold.Render("[" + key + "]")
		labelStyled := style.Bright.Render(label)
		hintStyled := style.Muted.Render(hint)
		if disabled {
			labelStyled = style.Muted.Render(label)
			hintStyled = style.Muted.Render("(no relay configured)")
		}
		return key + "  " + labelStyled + "  " + hintStyled
	}
	lines := []string{
		style.Dim.Width(width).Render("How do you want to add this contact?"),
		row("p", "paste invite", "paste an invite code or bundle", false),
		row("l", "lookup mailbox", "resolve via the trusted relay directory", !m.relayConfigured()),
		row("i", "invite exchange", "share a short code with a peer", !m.relayConfigured()),
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func (m *Model) renderAddContactPaste(width, modalHeight int) string {
	if m.addContact.preview != nil {
		preview := m.renderAddContactPreview(width)
		return preview
	}
	inputHeight := max(5, modalHeight-12)
	description := style.Dim.Width(width).Render("Paste a raw invite code or the full invite text. Pressing ctrl+s parses it and shows a preview before anything is saved.")
	input := m.renderAddContactEditor(width, inputHeight)
	return strings.Join([]string{description, input}, "\n\n")
}

func (m *Model) renderAddContactLookup(width int) string {
	description := style.Dim.Width(width).Render("Look up a contact in the trusted relay directory. The peer must have run `pando contact publish-directory` first.")
	label := style.Muted.Render("mailbox") + "  " + style.Bright.Render(m.addContact.value+style.CursorBlock.Render("█"))
	box := style.InputBorder.Width(width).Padding(0, 1).Render(label)
	status := ""
	if m.addContact.busy {
		status = style.Subtle.Render("looking up…")
	}
	parts := []string{description, box}
	if status != "" {
		parts = append(parts, status)
	}
	return strings.Join(parts, "\n\n")
}

func (m *Model) renderAddContactInviteChoice(width int) string {
	row := func(key, label, hint string) string {
		return style.Bold.Render("["+key+"]") + "  " + style.Bright.Render(label) + "  " + style.Muted.Render(hint)
	}
	lines := []string{
		style.Dim.Width(width).Render("Invite exchange — are you creating a new code or accepting one?"),
		row("s", "start", "generate a code and wait for the peer"),
		row("a", "accept", "enter a code the peer shared with you"),
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func (m *Model) renderAddContactInviteStart(width int) string {
	description := style.Dim.Width(width).Render("Share this code with the other person, then wait while they accept it. Press Esc to cancel, or `a` to switch to accepting their code instead.")
	codeLabel := style.Muted.Render("invite code")
	codeValue := style.Bright.Bold(true).Render(m.addContact.code)
	if m.addContact.code == "" {
		codeValue = style.Muted.Render("generating…")
	}
	codeBox := style.InputBorder.Width(width).Padding(0, 1).Render(codeLabel + "  " + codeValue)
	status := ""
	if m.addContact.busy {
		status = style.Subtle.Render("waiting for peer…")
	}
	parts := []string{description, codeBox}
	if status != "" {
		parts = append(parts, status)
	}
	return strings.Join(parts, "\n\n")
}

func (m *Model) renderAddContactInviteAccept(width int) string {
	description := style.Dim.Width(width).Render("Enter the code the other person shared. Press Enter to submit.")
	label := style.Muted.Render("code") + "  " + style.Bright.Render(m.addContact.value+style.CursorBlock.Render("█"))
	box := style.InputBorder.Width(width).Padding(0, 1).Render(label)
	status := ""
	if m.addContact.busy {
		status = style.Subtle.Render("waiting for peer…")
	}
	parts := []string{description, box}
	if status != "" {
		parts = append(parts, status)
	}
	return strings.Join(parts, "\n\n")
}

func (m *Model) renderAddContactPreview(width int) string {
	c := m.addContact.preview
	if c == nil {
		return ""
	}
	fp := c.Fingerprint()
	deviceCount := len(c.ActiveDevices())
	row := func(label, value string) string {
		return style.Muted.Render(label) + "  " + value
	}
	body := strings.Join([]string{
		style.Bold.Render("parsed invite"),
		row("account    ", style.PeerAccentStyle(fp).Bold(true).Render(c.AccountID)),
		row("fingerprint", style.Bright.Render(style.FormatFingerprint(fp))),
		row("devices    ", style.Bright.Render(fmt.Sprintf("%d active", deviceCount))),
	}, "\n")
	return lipgloss.NewStyle().Width(width).Render(body)
}

func (m *Model) renderAddContactEditor(width, height int) string {
	content := m.addContact.value
	if content == "" {
		content = style.Muted.Render("account: alice\nfingerprint: ...\ninvite-code: ...")
	} else {
		content += style.CursorBlock.Render("█")
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}
	visible := strings.Join(lines, "\n")
	meta := style.Subtle.Render(fmt.Sprintf("%d chars", len([]rune(m.addContact.value))))
	if len(m.addContact.value) >= addContactLimit {
		meta = style.StatusBad.Render(fmt.Sprintf("input limit reached (%d chars)", addContactLimit))
	}
	box := style.InputBorder.Width(width).Height(height).Padding(0, 1).Render(visible)
	return strings.Join([]string{box, meta}, "\n")
}

func (m *Model) addContactFooter() string {
	if m.addContact.busy {
		return "esc cancel"
	}
	switch m.addContact.mode {
	case addContactModeChooser:
		return "p paste   l lookup   i invite   esc cancel"
	case addContactModePaste:
		if m.addContact.preview != nil {
			return "ctrl+s import and verify   esc back"
		}
		return "enter newline  ctrl+s preview  ctrl+u clear  esc back"
	case addContactModeLookup:
		return "enter submit  ctrl+u clear  esc back"
	case addContactModeInviteChoice:
		return "s start  a accept  esc back"
	case addContactModeInviteStart:
		return "esc back"
	case addContactModeInviteAccept:
		return "enter submit  ctrl+u clear  esc back"
	}
	return ""
}
