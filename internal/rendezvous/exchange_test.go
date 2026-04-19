package rendezvous_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/elpdev/pando/internal/messaging"
	"github.com/elpdev/pando/internal/relayapi"
	"github.com/elpdev/pando/internal/rendezvous"
	"github.com/elpdev/pando/internal/store"
)

type fakeRelayClient struct {
	mu        sync.Mutex
	puts      []relayapi.RendezvousPayload
	payloads  map[string][]relayapi.RendezvousPayload
	getHook   func(id string) ([]relayapi.RendezvousPayload, error)
	putErr    error
	getErrors chan error
}

func newFakeRelay() *fakeRelayClient {
	return &fakeRelayClient{payloads: map[string][]relayapi.RendezvousPayload{}}
}

func (f *fakeRelayClient) PutRendezvousPayload(id string, p relayapi.RendezvousPayload) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.putErr != nil {
		return f.putErr
	}
	f.puts = append(f.puts, p)
	f.payloads[id] = append(f.payloads[id], p)
	return nil
}

func (f *fakeRelayClient) GetRendezvousPayloads(id string) ([]relayapi.RendezvousPayload, error) {
	if f.getHook != nil {
		return f.getHook(id)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]relayapi.RendezvousPayload(nil), f.payloads[id]...), nil
}

// seedPeer injects a peer's payload into the slot without going through Put.
func (f *fakeRelayClient) seedPeer(id string, p relayapi.RendezvousPayload) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.payloads[id] = append(f.payloads[id], p)
}

func newIdentity(t *testing.T, mailbox string) *messaging.Service {
	t.Helper()
	svc, _, err := messaging.New(store.NewClientStore(t.TempDir()), mailbox)
	if err != nil {
		t.Fatalf("new %s: %v", mailbox, err)
	}
	return svc
}

func TestExchangeSuccessReturnsPeerBundle(t *testing.T) {
	alice := newIdentity(t, "alice")
	bob := newIdentity(t, "bob")
	relay := newFakeRelay()

	code := "11111-22222"
	peerPayload, err := rendezvous.EncryptBundle(code, bob.Identity().InviteBundle())
	if err != nil {
		t.Fatalf("encrypt peer: %v", err)
	}
	relay.seedPeer(rendezvous.DeriveID(code), peerPayload)

	got, err := rendezvous.Exchange(context.Background(), rendezvous.PollConfig{
		Client:        relay,
		Code:          code,
		Self:          alice.Identity().InviteBundle(),
		SelfAccountID: alice.Identity().AccountID,
		PollEvery:     5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if got.AccountID != bob.Identity().AccountID {
		t.Fatalf("got account %q, want bob %q", got.AccountID, bob.Identity().AccountID)
	}
	if len(relay.puts) != 1 {
		t.Fatalf("expected 1 put, got %d", len(relay.puts))
	}
}

func TestExchangeIgnoresOwnPayload(t *testing.T) {
	alice := newIdentity(t, "alice")
	relay := newFakeRelay()

	code := "99999-00000"
	released := make(chan struct{})
	go func() {
		// After two polls, seed a peer so the loop can exit. This verifies the
		// loop doesn't return alice's own uploaded payload as a match.
		time.Sleep(20 * time.Millisecond)
		peer := newIdentity(t, "peer")
		peerPayload, _ := rendezvous.EncryptBundle(code, peer.Identity().InviteBundle())
		relay.seedPeer(rendezvous.DeriveID(code), peerPayload)
		close(released)
	}()

	bundle, err := rendezvous.Exchange(context.Background(), rendezvous.PollConfig{
		Client:        relay,
		Code:          code,
		Self:          alice.Identity().InviteBundle(),
		SelfAccountID: alice.Identity().AccountID,
		PollEvery:     5 * time.Millisecond,
	})
	<-released
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if bundle.AccountID == alice.Identity().AccountID {
		t.Fatal("Exchange returned self payload")
	}
}

func TestExchangeCancelledByContext(t *testing.T) {
	alice := newIdentity(t, "alice")
	relay := newFakeRelay()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := rendezvous.Exchange(ctx, rendezvous.PollConfig{
		Client:        relay,
		Code:          "12345-67890",
		Self:          alice.Identity().InviteBundle(),
		SelfAccountID: alice.Identity().AccountID,
		PollEvery:     5 * time.Millisecond,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestExchangeDeadlineExceededReturnsTimedOut(t *testing.T) {
	alice := newIdentity(t, "alice")
	relay := newFakeRelay()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := rendezvous.Exchange(ctx, rendezvous.PollConfig{
		Client:        relay,
		Code:          "12345-67890",
		Self:          alice.Identity().InviteBundle(),
		SelfAccountID: alice.Identity().AccountID,
		PollEvery:     5 * time.Millisecond,
	})
	if !errors.Is(err, rendezvous.ErrTimedOut) {
		t.Fatalf("expected ErrTimedOut, got %v", err)
	}
}

func TestExchangePropagatesPutError(t *testing.T) {
	alice := newIdentity(t, "alice")
	relay := newFakeRelay()
	relay.putErr = errors.New("boom")

	_, err := rendezvous.Exchange(context.Background(), rendezvous.PollConfig{
		Client:        relay,
		Code:          "12345-67890",
		Self:          alice.Identity().InviteBundle(),
		SelfAccountID: alice.Identity().AccountID,
		PollEvery:     5 * time.Millisecond,
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected put error, got %v", err)
	}
}

func TestExchangeSkipsUndecryptablePeerPayloads(t *testing.T) {
	alice := newIdentity(t, "alice")
	bob := newIdentity(t, "bob")
	relay := newFakeRelay()

	code := "88888-77777"
	wrong, _ := rendezvous.EncryptBundle("00000-00000", bob.Identity().InviteBundle())
	right, _ := rendezvous.EncryptBundle(code, bob.Identity().InviteBundle())
	id := rendezvous.DeriveID(code)
	relay.seedPeer(id, wrong)
	relay.seedPeer(id, right)

	bundle, err := rendezvous.Exchange(context.Background(), rendezvous.PollConfig{
		Client:        relay,
		Code:          code,
		Self:          alice.Identity().InviteBundle(),
		SelfAccountID: alice.Identity().AccountID,
		PollEvery:     5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if bundle.AccountID != bob.Identity().AccountID {
		t.Fatalf("expected bob's account, got %s", bundle.AccountID)
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	bob := newIdentity(t, "bob")
	payload, err := rendezvous.EncryptBundle("abcde-fghij", bob.Identity().InviteBundle())
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	got, err := rendezvous.DecryptBundle("ABCDE-FGHIJ", payload) // exercise normalization
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got.AccountID != bob.Identity().AccountID {
		t.Fatalf("decrypt: account mismatch")
	}
}

func TestDecryptBundleFailsWithWrongCode(t *testing.T) {
	bob := newIdentity(t, "bob")
	payload, err := rendezvous.EncryptBundle("right-code1", bob.Identity().InviteBundle())
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := rendezvous.DecryptBundle("wrong-codee", payload); err == nil {
		t.Fatal("expected decrypt failure with wrong code")
	}
}
