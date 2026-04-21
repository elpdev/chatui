package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/elpdev/pando/internal/config"
	"github.com/elpdev/pando/internal/ui/style"
)

type addRelayModal struct {
	open      bool
	editing   bool
	original  string
	inputs    []textinput.Model
	focused   int
	error     string
	maskToken bool
}

func newAddRelayModal() addRelayModal {
	name := textinput.New()
	name.Placeholder = "name"
	url := textinput.New()
	url.Placeholder = "ws://localhost:8080/ws"
	token := textinput.New()
	token.Placeholder = "optional token"
	token.EchoMode = textinput.EchoPassword
	token.EchoCharacter = '*'
	return addRelayModal{inputs: []textinput.Model{name, url, token}, maskToken: true}
}

func (m *addRelayModal) Open() tea.Cmd {
	*m = newAddRelayModal()
	m.open = true
	return m.inputs[0].Focus()
}

func (m *addRelayModal) OpenEdit(relay config.RelayProfile) tea.Cmd {
	*m = newAddRelayModal()
	m.open = true
	m.editing = true
	m.original = relay.Name
	m.inputs[0].SetValue(relay.Name)
	m.inputs[1].SetValue(relay.URL)
	m.inputs[2].SetValue(relay.Token)
	return m.inputs[0].Focus()
}

func (m *addRelayModal) Close() {
	*m = newAddRelayModal()
}

func (m *addRelayModal) Update(msg tea.Msg) (bool, tea.Cmd) {
	if !m.open {
		return false, nil
	}
	if _, ok := msg.(tea.KeyMsg); !ok {
		return false, nil
	}
	keyMsg := msg.(tea.KeyMsg)
	switch keyMsg.Type {
	case tea.KeyEsc:
		return true, func() tea.Msg { return addRelayClosedMsg{} }
	case tea.KeyTab, tea.KeyShiftTab, tea.KeyUp, tea.KeyDown:
		return true, m.moveFocus(keyMsg)
	case tea.KeyEnter:
		relay, err := m.profile()
		if err != nil {
			m.error = err.Error()
			return true, nil
		}
		if m.editing {
			return true, func() tea.Msg { return editRelaySavedMsg{original: m.original, relay: relay} }
		}
		return true, func() tea.Msg { return addRelaySavedMsg{relay: relay} }
	case tea.KeyCtrlT:
		m.maskToken = !m.maskToken
		if m.maskToken {
			m.inputs[2].EchoMode = textinput.EchoPassword
		} else {
			m.inputs[2].EchoMode = textinput.EchoNormal
		}
		return true, nil
	}
	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	m.error = ""
	return true, cmd
}

func (m *addRelayModal) moveFocus(msg tea.KeyMsg) tea.Cmd {
	m.inputs[m.focused].Blur()
	if msg.Type == tea.KeyShiftTab || msg.Type == tea.KeyUp {
		m.focused = (m.focused + len(m.inputs) - 1) % len(m.inputs)
	} else {
		m.focused = (m.focused + 1) % len(m.inputs)
	}
	return m.inputs[m.focused].Focus()
}

func (m addRelayModal) profile() (config.RelayProfile, error) {
	name := strings.TrimSpace(m.inputs[0].Value())
	url := strings.TrimSpace(m.inputs[1].Value())
	token := strings.TrimSpace(m.inputs[2].Value())
	if name == "" {
		return config.RelayProfile{}, fmt.Errorf("relay name is required")
	}
	if url == "" {
		return config.RelayProfile{}, fmt.Errorf("relay url is required")
	}
	return config.RelayProfile{Name: name, URL: url, Token: token}, nil
}

func (m addRelayModal) Overlay(width, height int) string {
	if !m.open {
		return ""
	}
	bodyWidth := max(1, paletteWidth(width)-6)
	lines := []string{
		style.PaletteMeta.Width(bodyWidth).Render("Save a named relay profile and switch to it immediately."),
		renderRelayInput(bodyWidth, "Name", m.inputs[0], m.focused == 0),
		renderRelayInput(bodyWidth, "URL", m.inputs[1], m.focused == 1),
		renderRelayInput(bodyWidth, "Token", m.inputs[2], m.focused == 2),
	}
	if m.error != "" {
		lines = append(lines, style.StatusBad.Width(bodyWidth).Render(m.error))
	}
	title := "Add Relay"
	subtitle := "Relay profiles persist to device config."
	footer := "tab move · enter save · ctrl+t show/hide token · esc cancel"
	if m.editing {
		title = "Edit Relay"
		subtitle = "Update a saved relay profile and keep the active relay in sync."
		footer = "tab move · enter save changes · ctrl+t show/hide token · esc cancel"
	}
	return renderPaletteOverlay(width, height, title, subtitle, []string{strings.Join(lines, "\n\n")}, footer)
}

func renderRelayInput(width int, label string, input textinput.Model, focused bool) string {
	input.Width = max(1, width-2)
	heading := style.Muted.Render(label)
	if focused {
		heading = style.Bright.Render(label)
	}
	return heading + "\n" + style.PaletteInput.Width(width).Padding(0, 1).Render(input.View())
}

func (m *Model) openAddRelayModal() tea.Cmd {
	m.input.Blur()
	return m.addRelay.Open()
}

func (m *Model) openEditRelayModal(name string) tea.Cmd {
	relay, ok := m.lookupRelayProfile(name)
	if !ok {
		m.pushToast(fmt.Sprintf("relay %s not found", name), ToastBad)
		return nil
	}
	m.input.Blur()
	return m.addRelay.OpenEdit(relay)
}

func (m *Model) closeAddRelayModal(keepStatus bool) {
	m.addRelay.Close()
	if !keepStatus {
		m.pushToast("add relay cancelled", ToastInfo)
	}
	if m.ui.focus == focusChat {
		m.input.Focus()
	}
}

func (m *Model) handleAddRelaySavedMsg(msg addRelaySavedMsg) (*Model, tea.Cmd) {
	if err := m.addRelayProfile(msg.relay); err != nil {
		m.addRelay.error = err.Error()
		return m, nil
	}
	m.closeAddRelayModal(true)
	m.pushToast(fmt.Sprintf("saved relay %s", msg.relay.Name), ToastInfo)
	return m, m.switchRelay(msg.relay.Name)
}

func (m *Model) handleEditRelaySavedMsg(msg editRelaySavedMsg) (*Model, tea.Cmd) {
	if err := m.updateRelayProfile(msg.original, msg.relay); err != nil {
		m.addRelay.error = err.Error()
		return m, nil
	}
	m.closeAddRelayModal(true)
	m.pushToast(fmt.Sprintf("updated relay %s", msg.relay.Name), ToastInfo)
	if m.relay.active == msg.original || m.relay.active == msg.relay.Name {
		return m, m.switchRelay(msg.relay.Name)
	}
	return m, nil
}

func (m *Model) handleAddRelayClosedMsg(msg addRelayClosedMsg) (*Model, tea.Cmd) {
	m.closeAddRelayModal(msg.keepStatus)
	return m, nil
}
