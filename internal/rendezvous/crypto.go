package rendezvous

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/elpdev/pando/internal/identity"
	"github.com/elpdev/pando/internal/relayapi"
)

const payloadLifetime = 10 * time.Minute

// EncryptBundle seals an invite bundle under a key derived from the code,
// returning a relay-ready RendezvousPayload.
func EncryptBundle(code string, bundle identity.InviteBundle) (relayapi.RendezvousPayload, error) {
	aead, err := newAEAD(code)
	if err != nil {
		return relayapi.RendezvousPayload{}, err
	}
	plaintext, err := json.Marshal(bundle)
	if err != nil {
		return relayapi.RendezvousPayload{}, fmt.Errorf("encode invite bundle: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return relayapi.RendezvousPayload{}, fmt.Errorf("generate rendezvous nonce: %w", err)
	}
	now := time.Now().UTC()
	return relayapi.RendezvousPayload{
		Ciphertext: base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, plaintext, nil)),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		CreatedAt:  now,
		ExpiresAt:  now.Add(payloadLifetime),
	}, nil
}

// DecryptBundle attempts to open a peer payload with the key derived from
// the code. Returns an error if the code is wrong, the payload is malformed,
// or the AEAD tag doesn't match.
func DecryptBundle(code string, payload relayapi.RendezvousPayload) (*identity.InviteBundle, error) {
	aead, err := newAEAD(code)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(payload.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode rendezvous nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode rendezvous ciphertext: %w", err)
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt rendezvous payload: %w", err)
	}
	var bundle identity.InviteBundle
	if err := json.Unmarshal(plaintext, &bundle); err != nil {
		return nil, fmt.Errorf("decode invite bundle: %w", err)
	}
	return &bundle, nil
}

func newAEAD(code string) (cipher.AEAD, error) {
	hash := sha256.Sum256([]byte("pando-rendezvous-v1\n" + NormalizeCode(code)))
	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return nil, fmt.Errorf("create rendezvous cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create rendezvous cipher: %w", err)
	}
	return aead, nil
}
