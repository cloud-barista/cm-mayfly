package common

import "testing"

func TestMaskSecret(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"empty is distinguishable from set", "", "(empty)"},
		{"one char", "a", "***"},
		{"short values give nothing away", "abcdef", "***"},
		{"just over the threshold", "abcdefg", "ab***"},
		{"typical password", "p4ssw0rd-with-more", "p4***"},
		{"long opaque token", "ey-token-shaped-value-for-masking-only", "ey***"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := MaskSecret(tc.in); got != tc.want {
				t.Errorf("MaskSecret(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// The whole point is that the secret does not appear in the output. Checked
// separately from the table so the property is stated rather than implied by a
// list of expected strings.
func TestMaskSecretNeverContainsTheWholeValue(t *testing.T) {
	for _, secret := range []string{
		"a",
		"short",
		"abcdefg",
		"correct-horse-battery-staple",
		"s.vault-token-shaped-value-for-masking-only",
	} {
		got := MaskSecret(secret)
		if got == secret {
			t.Errorf("MaskSecret(%q) returned the secret unchanged", secret)
		}
		if len(got) >= len(secret) && len(secret) > 6 {
			t.Errorf("MaskSecret(%q) = %q is not shorter than the secret", secret, got)
		}
	}
}

// A multi-byte value must not be cut in the middle of a character — the prefix
// is taken in runes, so the result stays printable.
func TestMaskSecretMultiByte(t *testing.T) {
	got := MaskSecret("비밀번호입니다")
	if want := "비밀***"; got != want {
		t.Errorf("MaskSecret(multi-byte) = %q, want %q", got, want)
	}
	for _, r := range got {
		if r == '�' {
			t.Errorf("MaskSecret produced a replacement character: %q", got)
		}
	}
}
