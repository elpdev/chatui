package relay

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/elpdev/pando/internal/protocol"
	"github.com/elpdev/pando/internal/relayapi"
	bbolt "go.etcd.io/bbolt"
)

func filterExpired(queue []protocol.Envelope, now time.Time) []protocol.Envelope {
	return filterInPlace(queue, func(envelope protocol.Envelope) bool {
		return envelope.ExpiresAt.IsZero() || envelope.ExpiresAt.After(now)
	})
}

func filterExpiredRendezvous(payloads []relayapi.RendezvousPayload, now time.Time) []relayapi.RendezvousPayload {
	return filterInPlace(payloads, func(payload relayapi.RendezvousPayload) bool {
		return payload.ExpiresAt.IsZero() || payload.ExpiresAt.After(now)
	})
}

func filterInPlace[T any](values []T, keep func(T) bool) []T {
	filtered := values[:0]
	for _, value := range values {
		if !keep(value) {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func validateQueueLimits(queue []protocol.Envelope, next protocol.Envelope, limits QueueLimits) error {
	if limits.MaxMessages > 0 && len(queue)+1 > limits.MaxMessages {
		return ErrQueueFull
	}
	if limits.MaxBytes <= 0 {
		return nil
	}
	totalBytes := envelopeSize(next)
	for _, envelope := range queue {
		totalBytes += envelopeSize(envelope)
	}
	if totalBytes > limits.MaxBytes {
		return ErrQueueFull
	}
	return nil
}

type mailboxOwnerSync struct {
	getAccount  func(mailbox string) (string, bool, error)
	getClaim    func(mailbox string) ([]byte, bool, error)
	putOwner    func(mailbox, account string, key []byte) error
	deleteOwner func(mailbox string) error
}

func (s mailboxOwnerSync) sync(previous, next relayapi.SignedDirectoryEntry) error {
	active := activeDirectoryDevices(next)
	for mailbox, key := range active {
		account, ok, err := s.getAccount(mailbox)
		if err != nil {
			return err
		}
		if ok && account != next.Entry.Mailbox {
			return ErrMailboxClaimConflict
		}
		existingKey, ok, err := s.getClaim(mailbox)
		if err != nil {
			return err
		}
		if ok && !bytes.Equal(existingKey, key) {
			return ErrMailboxClaimConflict
		}
	}
	for mailbox := range activeDirectoryDevices(previous) {
		account, ok, err := s.getAccount(mailbox)
		if err != nil {
			return err
		}
		if ok && account == previous.Entry.Mailbox {
			if err := s.deleteOwner(mailbox); err != nil {
				return err
			}
		}
	}
	for mailbox, key := range active {
		if err := s.putOwner(mailbox, next.Entry.Mailbox, key); err != nil {
			return err
		}
	}
	return nil
}

func memoryMailboxOwnerSync(store *MemoryQueueStore) mailboxOwnerSync {
	return mailboxOwnerSync{
		getAccount: func(mailbox string) (string, bool, error) {
			account, ok := store.accounts[mailbox]
			return account, ok, nil
		},
		getClaim: func(mailbox string) ([]byte, bool, error) {
			key, ok := store.claims[mailbox]
			return key, ok, nil
		},
		putOwner: func(mailbox, account string, key []byte) error {
			store.accounts[mailbox] = account
			store.claims[mailbox] = append([]byte(nil), key...)
			return nil
		},
		deleteOwner: func(mailbox string) error {
			delete(store.accounts, mailbox)
			delete(store.claims, mailbox)
			return nil
		},
	}
}

func boltMailboxOwnerSync(tx *bbolt.Tx) mailboxOwnerSync {
	claimBucket := tx.Bucket(mailboxClaimBucket)
	accountBucket := tx.Bucket(mailboxDirectoryBucket)
	return mailboxOwnerSync{
		getAccount: func(mailbox string) (string, bool, error) {
			account := accountBucket.Get([]byte(mailbox))
			if len(account) == 0 {
				return "", false, nil
			}
			return string(account), true, nil
		},
		getClaim: func(mailbox string) ([]byte, bool, error) {
			key := claimBucket.Get([]byte(mailbox))
			if len(key) == 0 {
				return nil, false, nil
			}
			return key, true, nil
		},
		putOwner: func(mailbox, account string, key []byte) error {
			if err := accountBucket.Put([]byte(mailbox), []byte(account)); err != nil {
				return err
			}
			return claimBucket.Put([]byte(mailbox), append([]byte(nil), key...))
		},
		deleteOwner: func(mailbox string) error {
			if err := accountBucket.Delete([]byte(mailbox)); err != nil {
				return err
			}
			return claimBucket.Delete([]byte(mailbox))
		},
	}
}

func activeDirectoryDevices(entry relayapi.SignedDirectoryEntry) map[string][]byte {
	devices := make(map[string][]byte)
	if entry.Entry.Mailbox == "" {
		return devices
	}
	for _, device := range entry.Entry.Bundle.Devices {
		if device.Revoked {
			continue
		}
		devices[device.Mailbox] = append([]byte(nil), device.SigningPublic...)
	}
	return devices
}

func boltGet[T any](bucket *bbolt.Bucket, key []byte, dst *T) (bool, error) {
	current := bucket.Get(key)
	if len(current) == 0 {
		return false, nil
	}
	if err := json.Unmarshal(current, dst); err != nil {
		return false, err
	}
	return true, nil
}

func boltPut[T any](bucket *bbolt.Bucket, key []byte, value T) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return bucket.Put(key, encoded)
}
