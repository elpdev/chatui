package rendezvous

import (
	"strings"
	"testing"
)

func TestNormalizeCodeStripsFormatting(t *testing.T) {
	want := "1234567890"
	for _, input := range []string{"1234567890", "12345-67890", "  12345 67890  ", "12345-67890\n", "1234567890"} {
		if got := NormalizeCode(input); got != want {
			t.Fatalf("NormalizeCode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDeriveIDStableAcrossFormatting(t *testing.T) {
	a := DeriveID("12345-67890")
	b := DeriveID("  12345 67890 ")
	if a != b {
		t.Fatalf("expected stable rendezvous id across formatting, got %q and %q", a, b)
	}
	if strings.Contains(a, "=") {
		t.Fatalf("rendezvous id should use raw URL encoding, got %q", a)
	}
}

func TestGenerateCodeFormat(t *testing.T) {
	code, err := GenerateCode()
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if len(code) != 11 || code[5] != '-' {
		t.Fatalf("unexpected code shape %q", code)
	}
	for _, r := range code {
		if r == '-' {
			continue
		}
		if r < '0' || r > '9' {
			t.Fatalf("non-digit rune %q in code %q", r, code)
		}
	}
}
