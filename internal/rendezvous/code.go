// Package rendezvous implements the short-code rendezvous used to exchange
// invite bundles over a relay without a pre-shared contact.
//
// Two sides independently derive a rendezvous ID and an AES-GCM key from the
// same human-friendly code (e.g. "12345-67890"), encrypt their invite bundle
// under that key, upload it to the relay, and poll for the peer's payload.
// Because the code is the only shared secret and it never leaves the devices,
// the relay only ever sees opaque ciphertext.
package rendezvous

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// GenerateCode returns a random ten-digit code formatted as "xxxxx-xxxxx".
func GenerateCode() (string, error) {
	digits := make([]byte, 10)
	random := make([]byte, len(digits))
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate invite code: %w", err)
	}
	for idx := range digits {
		digits[idx] = '0' + (random[idx] % 10)
	}
	return fmt.Sprintf("%s-%s", digits[:5], digits[5:]), nil
}

// NormalizeCode lowercases the code and strips whitespace and dashes so that
// "12345-67890" and "12345 67890" derive the same rendezvous ID and key.
func NormalizeCode(code string) string {
	code = strings.TrimSpace(strings.ToLower(code))
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}

// DeriveID returns the rendezvous slot ID both sides upload to and poll from.
// Domain-separated from the AES key so the relay can't brute-force either from
// observing the other.
func DeriveID(code string) string {
	hash := sha256.Sum256([]byte("pando-rendezvous-id-v1\n" + NormalizeCode(code)))
	return base64.RawURLEncoding.EncodeToString(hash[:16])
}
