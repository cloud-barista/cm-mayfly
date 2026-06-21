package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveEnvRef(t *testing.T) {
	// non-${VAR} literal is returned unchanged (backward compatible).
	if v, unset := ResolveEnvRef("default"); v != "default" || unset {
		t.Fatalf("literal: got %q unset=%v, want default/false", v, unset)
	}
	if v, unset := ResolveEnvRef(""); v != "" || unset {
		t.Fatalf("empty literal: got %q unset=%v", v, unset)
	}

	// process OS env has priority and resolves the reference.
	t.Setenv("MAYFLY_TEST_AUTH", "osval")
	if v, unset := ResolveEnvRef("${MAYFLY_TEST_AUTH}"); v != "osval" || unset {
		t.Fatalf("os env: got %q unset=%v, want osval/false", v, unset)
	}

	// reference to an unset var → empty value + unset=true (no silent default).
	if v, unset := ResolveEnvRef("${MAYFLY_DEFINITELY_UNSET_XYZ_123}"); v != "" || !unset {
		t.Fatalf("unset ref: got %q unset=%v, want \"\"/true", v, unset)
	}

	// malformed ref is treated as a literal.
	if v, unset := ResolveEnvRef("${"); v != "${" || unset {
		t.Fatalf("malformed: got %q unset=%v", v, unset)
	}
}

func TestParseDotEnv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	content := "FOO=bar\nexport BAZ=\"qux\"\n# comment line\nEMPTY=\nSINGLE='abc'\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseDotEnv(p)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" {
		t.Errorf("FOO=%q want bar", m["FOO"])
	}
	if m["BAZ"] != "qux" {
		t.Errorf("BAZ=%q want qux (export + dquotes stripped)", m["BAZ"])
	}
	if m["SINGLE"] != "abc" {
		t.Errorf("SINGLE=%q want abc (squotes stripped)", m["SINGLE"])
	}
	if v, ok := m["EMPTY"]; !ok || v != "" {
		t.Errorf("EMPTY=%q ok=%v want \"\"/true", v, ok)
	}
}
