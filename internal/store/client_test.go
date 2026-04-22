package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/elpdev/pando/internal/identity"
)

func TestMarkContactVerified(t *testing.T) {
	clientStore := NewClientStore(t.TempDir())
	contactID, err := identity.New("bob")
	if err != nil {
		t.Fatalf("new identity: %v", err)
	}
	contact, err := identity.ContactFromInvite(contactID.InviteBundle())
	if err != nil {
		t.Fatalf("contact from invite: %v", err)
	}
	if err := clientStore.SaveContact(contact); err != nil {
		t.Fatalf("save contact: %v", err)
	}

	verified, err := clientStore.MarkContactVerified("bob", true)
	if err != nil {
		t.Fatalf("mark verified: %v", err)
	}
	if !verified.Verified {
		t.Fatalf("expected contact to be verified")
	}
}

func TestSaveAttachmentRejectsTraversalComponents(t *testing.T) {
	clientStore := NewClientStore(t.TempDir())
	id, err := identity.New("alice")
	if err != nil {
		t.Fatalf("new identity: %v", err)
	}
	if _, err := clientStore.SaveAttachment(id, "../bob", "file-1", "photo.png", []byte("hello")); err == nil {
		t.Fatal("expected traversal mailbox to be rejected")
	}
	if _, err := clientStore.SaveAttachment(id, "bob", "../file-1", "photo.png", []byte("hello")); err == nil {
		t.Fatal("expected traversal attachment id to be rejected")
	}
}

func TestSaveAttachmentReplacesSpacesInFilename(t *testing.T) {
	clientStore := NewClientStore(t.TempDir())
	id, err := identity.New("alice")
	if err != nil {
		t.Fatalf("new identity: %v", err)
	}
	path, err := clientStore.SaveAttachment(id, "bob", "file-1", "my photo clip.m4a", []byte("hello"))
	if err != nil {
		t.Fatalf("save attachment: %v", err)
	}
	if got, want := filepath.Base(path), "file-1-my_photo_clip.m4a"; got != want {
		t.Fatalf("unexpected saved filename: got %q want %q", got, want)
	}
}

func TestSaveAttachmentEncryptsBytesAndReadAttachmentDecrypts(t *testing.T) {
	clientStore := NewClientStore(t.TempDir())
	id, err := identity.New("alice")
	if err != nil {
		t.Fatalf("new identity: %v", err)
	}
	original := []byte("hello attachment")
	path, err := clientStore.SaveAttachment(id, "bob", "file-1", "photo.png", original)
	if err != nil {
		t.Fatalf("save attachment: %v", err)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	if bytes.Equal(onDisk, original) {
		t.Fatal("expected attachment bytes on disk to be encrypted")
	}
	plaintext, err := clientStore.ReadAttachment(id, path)
	if err != nil {
		t.Fatalf("read attachment plaintext: %v", err)
	}
	if !bytes.Equal(plaintext, original) {
		t.Fatal("expected decrypted attachment bytes to match original")
	}
}

func TestSaveIdentityCompactsOldRevokedDevices(t *testing.T) {
	clientStore := NewClientStore(t.TempDir())
	id := mustIdentityWithSecondDevice(t, "alice", "alice-phone")
	if err := id.RevokeDevice("alice-phone"); err != nil {
		t.Fatalf("revoke device: %v", err)
	}
	id.Devices[1].RevokedAt = time.Now().UTC().Add(-31 * 24 * time.Hour)

	if err := clientStore.SaveIdentity(id); err != nil {
		t.Fatalf("save identity: %v", err)
	}
	loaded, err := clientStore.LoadIdentity()
	if err != nil {
		t.Fatalf("load identity: %v", err)
	}
	if len(loaded.Devices) != 1 {
		t.Fatalf("expected old revoked device to be compacted, got %d devices", len(loaded.Devices))
	}
}

func TestSaveIdentityKeepsRecentRevokedDevices(t *testing.T) {
	clientStore := NewClientStore(t.TempDir())
	id := mustIdentityWithSecondDevice(t, "alice", "alice-phone")
	if err := id.RevokeDevice("alice-phone"); err != nil {
		t.Fatalf("revoke device: %v", err)
	}
	id.Devices[1].RevokedAt = time.Now().UTC().Add(-7 * 24 * time.Hour)

	if err := clientStore.SaveIdentity(id); err != nil {
		t.Fatalf("save identity: %v", err)
	}
	loaded, err := clientStore.LoadIdentity()
	if err != nil {
		t.Fatalf("load identity: %v", err)
	}
	if len(loaded.Devices) != 2 {
		t.Fatalf("expected recent revoked device to be retained, got %d devices", len(loaded.Devices))
	}
}

func TestCompactRevokedDevicesKeepsCurrentDevice(t *testing.T) {
	id, err := identity.New("alice")
	if err != nil {
		t.Fatalf("new identity: %v", err)
	}
	id.Devices[0].Revoked = true
	id.Devices[0].RevokedAt = time.Now().UTC().Add(-31 * 24 * time.Hour)

	id.CompactRevokedDevices(time.Now().UTC().Add(-30 * 24 * time.Hour))

	if len(id.Devices) != 1 {
		t.Fatalf("expected current device to be retained, got %d devices", len(id.Devices))
	}
}

func mustIdentityWithSecondDevice(t *testing.T, accountID, mailbox string) *identity.Identity {
	t.Helper()
	id, err := identity.New(accountID)
	if err != nil {
		t.Fatalf("new identity: %v", err)
	}
	pending, err := identity.NewPendingEnrollment(accountID, mailbox)
	if err != nil {
		t.Fatalf("new pending enrollment: %v", err)
	}
	approval, err := id.Approve(pending.Request())
	if err != nil {
		t.Fatalf("approve enrollment: %v", err)
	}
	if _, err := pending.Complete(*approval); err != nil {
		t.Fatalf("complete enrollment: %v", err)
	}
	return id
}
