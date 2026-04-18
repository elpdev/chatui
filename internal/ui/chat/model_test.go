package chat

import (
	"context"
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/elpdev/pando/internal/messaging"
	"github.com/elpdev/pando/internal/protocol"
	"github.com/elpdev/pando/internal/store"
	"github.com/elpdev/pando/internal/transport"
)

type stubClient struct{}

func (stubClient) Connect(context.Context) error { return nil }
func (stubClient) Events() <-chan transport.Event {
	ch := make(chan transport.Event)
	return ch
}
func (stubClient) Send(protocol.Envelope) error { return nil }
func (stubClient) Close() error                 { return nil }

func TestAuthFailureKeepsHistoryVisibleAndStopsReconnect(t *testing.T) {
	clientStore := store.NewClientStore(t.TempDir())
	service, _, err := messaging.New(clientStore, "alice")
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if err := service.SaveReceived("bobn", "hello from history", time.Now().UTC()); err != nil {
		t.Fatalf("save received history: %v", err)
	}

	model := New(Deps{
		Client:           stubClient{},
		Messaging:        service,
		Mailbox:          "alice",
		RecipientMailbox: "bobn",
		RelayURL:         "wss://pandorelay.network/ws",
	})

	initCmd := model.Init()
	if initCmd == nil {
		t.Fatal("expected init command")
	}
	if len(model.messages) < 4 {
		t.Fatalf("expected local history to be loaded, got %d messages", len(model.messages))
	}

	updated, cmd := model.Update(clientEventMsg(transport.Event{Err: fmt.Errorf("%w: check relay token", transport.ErrUnauthorized)}))
	if updated != model {
		t.Fatal("expected update to mutate the existing model")
	}
	if cmd != nil {
		t.Fatal("expected no reconnect command after auth failure")
	}
	if !model.authFailed {
		t.Fatal("expected auth failure state")
	}
	if model.connected {
		t.Fatal("expected model to remain disconnected")
	}
	if model.input.Placeholder != "Relay auth failed. Restart with --relay-token" {
		t.Fatalf("unexpected placeholder: %q", model.input.Placeholder)
	}
	if model.status != "relay auth failed: relay unauthorized: check relay token" {
		t.Fatalf("unexpected status: %q", model.status)
	}

	model.input.SetValue("hi")
	_, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if model.status != "cannot send: relay auth failed; restart with --relay-token" {
		t.Fatalf("unexpected send status: %q", model.status)
	}
	if model.input.Value() != "hi" {
		t.Fatalf("expected input to remain unchanged, got %q", model.input.Value())
	}
	if len(model.messages) < 4 {
		t.Fatalf("expected local history to remain visible, got %d messages", len(model.messages))
	}
}
