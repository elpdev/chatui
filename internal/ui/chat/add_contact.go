package chat

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/elpdev/pando/internal/identity"
	"github.com/elpdev/pando/internal/ui/style"
)

type addContactState struct {
	value     string
	error     string
	importing bool
	open      bool
	preview   *identity.Contact
}

func (m *Model) openAddContactModal() {
	m.addContact.open = true
	m.addContact.error = ""
	m.addContact.importing = false
	m.addContact.value = ""
	m.addContact.preview = nil
	m.input.Blur()
}

func (m *Model) closeAddContactModal(keepStatus bool) {
	m.addContact.open = false
	m.addContact.importing = false
	m.addContact.error = ""
	m.addContact.value = ""
	m.addContact.preview = nil
	if !keepStatus {
		m.pushToast("add contact cancelled", ToastInfo)
	}
	m.input.Focus()
}

func (m *Model) handleAddContactKey(msg tea.KeyMsg) (*Model, tea.Cmd) {
	if m.addContact.preview != nil {
		switch msg.Type {
		case tea.KeyEsc:
			if m.addContact.importing {
				return m, nil
			}
			m.addContact.preview = nil
			m.addContact.error = ""
			return m, nil
		case tea.KeyCtrlS:
			if m.addContact.importing {
				return m, nil
			}
			m.addContact.error = ""
			m.addContact.importing = true
			return m, m.importContactCmd(strings.TrimSpace(m.addContact.value))
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.closeAddContactModal(false)
		return m, nil
	case tea.KeyCtrlS:
		if m.addContact.importing {
			return m, nil
		}
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
		return m, nil
	case tea.KeyBackspace, tea.KeyCtrlH, tea.KeyDelete:
		m.deleteAddContactRune()
		return m, nil
	case tea.KeyCtrlU:
		m.addContact.value = ""
		m.addContact.error = ""
		return m, nil
	case tea.KeyRunes:
		m.appendAddContactText(string(msg.Runes))
		return m, nil
	default:
		return m, nil
	}
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

func (m *Model) importContactCmd(text string) tea.Cmd {
	return func() tea.Msg {
		contact, err := m.messaging.ImportContactInviteText(text, true)
		if err != nil {
			return addContactResultMsg{err: err}
		}
		return addContactResultMsg{contact: contact}
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

	if m.addContact.preview != nil {
		parts = append(parts, m.renderAddContactPreview(bodyWidth))
		footer := "ctrl+s import and verify   esc back"
		if m.addContact.importing {
			footer = "importing contact..."
		}
		if m.addContact.error != "" {
			parts = append(parts, style.StatusBad.Width(bodyWidth).Render(m.addContact.error))
		}
		parts = append(parts, style.Subtle.Render(footer))
	} else {
		inputHeight := max(5, modalHeight-10)
		description := style.Dim.Width(bodyWidth).Render("Paste a raw invite code or the full invite text. Pressing ctrl+s parses it and shows a preview before anything is saved.")
		input := m.renderAddContactEditor(bodyWidth, inputHeight)
		parts = append(parts, description, input)
		if m.addContact.error != "" {
			parts = append(parts, style.StatusBad.Width(bodyWidth).Render(m.addContact.error))
		}
		footer := "enter newline  ctrl+s preview  ctrl+u clear  esc cancel"
		parts = append(parts, style.Subtle.Render(footer))
	}

	modal := style.Modal.Width(modalWidth).Padding(1, 2).Render(strings.Join(parts, "\n\n"))
	background := style.Faint.Render(base)
	return strings.Join([]string{background, lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)}, "\n")
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
