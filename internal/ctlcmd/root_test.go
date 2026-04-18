package ctlcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elpdev/pando/internal/identity"
	"github.com/elpdev/pando/internal/store"
)

func TestEjectForce(t *testing.T) {
	dataDir := t.TempDir()
	mailbox := "alice"
	clientStore := store.NewClientStore(dataDir)
	if _, _, err := clientStore.LoadOrCreateIdentity(mailbox); err != nil {
		t.Fatalf("create identity: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "identity.json")); err != nil {
		t.Fatalf("expected identity file to exist before eject: %v", err)
	}

	if err := runEject([]string{"-mailbox", mailbox, "-data-dir", dataDir, "-force"}); err != nil {
		t.Fatalf("eject: %v", err)
	}

	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("expected data dir to be removed after eject")
	}
}

func TestEjectConfirmation(t *testing.T) {
	dataDir := t.TempDir()
	mailbox := "bob"
	clientStore := store.NewClientStore(dataDir)
	if _, _, err := clientStore.LoadOrCreateIdentity(mailbox); err != nil {
		t.Fatalf("create identity: %v", err)
	}

	// Patch stdin to simulate user typing the mailbox name.
	origStdin := os.Stdin
	t.Cleanup(func() { os.Stdin = origStdin })
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = r
	if _, err := w.WriteString(mailbox + "\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	w.Close()

	if err := runEject([]string{"-mailbox", mailbox, "-data-dir", dataDir}); err != nil {
		t.Fatalf("eject: %v", err)
	}

	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("expected data dir to be removed after eject")
	}
}

func TestEjectConfirmationAbort(t *testing.T) {
	dataDir := t.TempDir()
	mailbox := "carol"
	clientStore := store.NewClientStore(dataDir)
	if _, _, err := clientStore.LoadOrCreateIdentity(mailbox); err != nil {
		t.Fatalf("create identity: %v", err)
	}

	origStdin := os.Stdin
	t.Cleanup(func() { os.Stdin = origStdin })
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = r
	if _, err := w.WriteString("wrong-mailbox\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	w.Close()

	err = runEject([]string{"-mailbox", mailbox, "-data-dir", dataDir})
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("expected aborted error, got %v", err)
	}

	// Data dir should still exist.
	if _, statErr := os.Stat(dataDir); statErr != nil {
		t.Fatalf("expected data dir to survive aborted eject: %v", statErr)
	}
}

func TestInviteCodeRoundTrip(t *testing.T) {
	id, err := identity.New("alice")
	if err != nil {
		t.Fatalf("new identity: %v", err)
	}
	code, err := encodeInviteCode(id.InviteBundle())
	if err != nil {
		t.Fatalf("encode invite code: %v", err)
	}
	bundle, err := decodeInviteCode(code)
	if err != nil {
		t.Fatalf("decode invite code: %v", err)
	}
	if bundle.AccountID != id.AccountID {
		t.Fatalf("expected account id %s, got %s", id.AccountID, bundle.AccountID)
	}
	if len(bundle.Devices) != len(id.InviteBundle().Devices) {
		t.Fatalf("expected %d devices, got %d", len(id.InviteBundle().Devices), len(bundle.Devices))
	}
}
