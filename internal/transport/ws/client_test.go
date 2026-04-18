package ws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elpdev/pando/internal/transport"
)

func TestConnectReturnsUnauthorizedErrorForBadHandshake(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "relay auth token is required", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient("ws"+server.URL[len("http"):], "wrong-token", "alice")
	err := client.Connect(context.Background())
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	if !errors.Is(err, transport.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}
