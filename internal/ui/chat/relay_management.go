package chat

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/elpdev/pando/internal/config"
)

func (m *Model) addRelayProfile(relay config.RelayProfile) error {
	relay.Name = strings.TrimSpace(relay.Name)
	relay.URL = strings.TrimSpace(relay.URL)
	if relay.Name == "" || relay.URL == "" {
		return fmt.Errorf("relay name and url are required")
	}
	for _, existing := range m.relay.profiles {
		if existing.Name == relay.Name {
			return fmt.Errorf("relay %q already exists", relay.Name)
		}
	}
	m.relay.profiles = append(m.relay.profiles, relay)
	if err := m.persistRelayProfiles(m.relay.active); err != nil {
		m.relay.profiles = m.relay.profiles[:len(m.relay.profiles)-1]
		return err
	}
	return nil
}

func (m *Model) updateRelayProfile(original string, relay config.RelayProfile) error {
	original = strings.TrimSpace(original)
	relay.Name = strings.TrimSpace(relay.Name)
	relay.URL = strings.TrimSpace(relay.URL)
	if original == "" || relay.Name == "" || relay.URL == "" {
		return fmt.Errorf("relay name and url are required")
	}
	updated := false
	previous := append([]config.RelayProfile(nil), m.relay.profiles...)
	for i, existing := range m.relay.profiles {
		if existing.Name != original {
			if existing.Name == relay.Name {
				return fmt.Errorf("relay %q already exists", relay.Name)
			}
			continue
		}
		m.relay.profiles[i] = relay
		updated = true
	}
	if !updated {
		return fmt.Errorf("relay %q not found", original)
	}
	active := m.relay.active
	if active == original {
		active = relay.Name
	}
	if err := m.persistRelayProfiles(active); err != nil {
		m.relay.profiles = previous
		return err
	}
	if m.relay.active == original {
		m.relay.active = relay.Name
	}
	if m.relay.active == relay.Name {
		m.relay.url = relay.URL
		m.relay.token = relay.Token
		m.relay.client = nil
		m.updateDirectoryClient()
	}
	return nil
}

func (m *Model) removeRelayProfile(name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if name == "" {
		m.pushToast("relay name is required", ToastBad)
		return nil
	}
	if len(m.relay.profiles) <= 1 {
		m.pushToast("cannot remove the last saved relay", ToastBad)
		return nil
	}
	filtered := make([]config.RelayProfile, 0, len(m.relay.profiles)-1)
	removed := false
	for _, relay := range m.relay.profiles {
		if relay.Name == name {
			removed = true
			continue
		}
		filtered = append(filtered, relay)
	}
	if !removed {
		m.pushToast(fmt.Sprintf("relay %s not found", name), ToastBad)
		return nil
	}
	previous := append([]config.RelayProfile(nil), m.relay.profiles...)
	m.relay.profiles = filtered
	active := m.relay.active
	if active == name {
		active = filtered[0].Name
	}
	if err := m.persistRelayProfiles(active); err != nil {
		m.relay.profiles = previous
		m.pushToast(fmt.Sprintf("remove relay failed: %v", err), ToastBad)
		return nil
	}
	if m.relay.active == name {
		m.pushToast(fmt.Sprintf("removed relay %s", name), ToastInfo)
		return m.switchRelay(active)
	}
	m.pushToast(fmt.Sprintf("removed relay %s", name), ToastInfo)
	return nil
}

func (m *Model) switchRelay(name string) tea.Cmd {
	profile, ok := m.lookupRelayProfile(name)
	if !ok {
		m.pushToast(fmt.Sprintf("relay %s not found", name), ToastBad)
		return nil
	}
	if err := m.persistRelayProfiles(profile.Name); err != nil {
		m.pushToast(fmt.Sprintf("switch relay failed: %v", err), ToastBad)
		return nil
	}
	m.relay.active = profile.Name
	m.relay.url = profile.URL
	m.relay.token = profile.Token
	m.relay.client = nil
	m.updateDirectoryClient()
	oldClient := m.client
	m.client = m.relay.transportFactory(profile.URL, profile.Token)
	m.conn.connecting = true
	m.conn.connected = false
	m.conn.disconnected = false
	m.conn.authFailed = false
	m.conn.reconnectAttempt = 0
	m.conn.reconnectDelay = 0
	m.conn.status = fmt.Sprintf("switching relay to %s", profile.Name)
	m.syncInputPlaceholder()
	if oldClient != nil {
		_ = oldClient.Close()
	}
	m.pushToast(fmt.Sprintf("switched relay to %s", profile.Name), ToastInfo)
	return tea.Batch(m.connectCmd(), m.waitForEvent())
}

func (m *Model) lookupRelayProfile(name string) (config.RelayProfile, bool) {
	for _, relay := range m.relay.profiles {
		if relay.Name == name {
			return relay, true
		}
	}
	return config.RelayProfile{}, false
}

func (m *Model) persistRelayProfiles(active string) error {
	if m.relay.saveProfiles == nil {
		return nil
	}
	return m.relay.saveProfiles(append([]config.RelayProfile(nil), m.relay.profiles...), active)
}

func (m *Model) updateDirectoryClient() {
	if strings.TrimSpace(m.relay.url) == "" {
		m.messaging.SetDirectoryClient(nil)
		return
	}
	client, err := m.relay.clientFactory(m.relay.url, m.relay.token)
	if err != nil {
		m.messaging.SetDirectoryClient(nil)
		return
	}
	m.relay.client = client
	m.messaging.SetDirectoryClient(client)
}
