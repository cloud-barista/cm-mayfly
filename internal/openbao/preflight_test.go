package openbao

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- signal helpers ---------------------------------------------------------

func TestInitFileShapeParsesRootToken(t *testing.T) {
	raw := `{"keys":["abc123"],"keys_base64":["Zm9v"],"root_token":"test-root-token-not-real"}`
	var s initFileShape
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.firstKey() != "abc123" {
		t.Errorf("firstKey = %q, want abc123", s.firstKey())
	}
	if s.RootToken != "test-root-token-not-real" {
		t.Errorf("RootToken = %q, want test-root-token-not-real", s.RootToken)
	}
}

// fixtureRoot builds a temp mayfly root with conf/docker/{.env,data/openbao/...}
// and chdir's into it for the duration of the test.
func fixtureRoot(t *testing.T, token, initJSON string, dataPopulated bool) {
	t.Helper()
	root := t.TempDir()
	dockerDir := filepath.Join(root, "conf", "docker")
	secretsDir := filepath.Join(dockerDir, "data", "openbao", "secrets")
	dataDir := filepath.Join(dockerDir, "data", "openbao", "data")
	for _, d := range []string{secretsDir, dataDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	env := ""
	if token != "" {
		env = "VAULT_TOKEN=" + token + "\nTUMBLEBUG_DB_PASSWORD=keepme\n"
	} else {
		env = "VAULT_TOKEN=\nTUMBLEBUG_DB_PASSWORD=keepme\n"
	}
	if err := os.WriteFile(filepath.Join(dockerDir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	if initJSON != "" {
		if err := os.WriteFile(filepath.Join(secretsDir, "openbao-init.json"), []byte(initJSON), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if dataPopulated {
		if err := os.WriteFile(filepath.Join(dataDir, "core"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
}

func TestDataDirState(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		t.Chdir(t.TempDir())
		pop, known := dataDirState()
		if pop || !known {
			t.Errorf("absent dir: pop=%v known=%v, want false,true", pop, known)
		}
	})
	t.Run("empty", func(t *testing.T) {
		fixtureRoot(t, "", "", false)
		pop, known := dataDirState()
		if pop || !known {
			t.Errorf("empty dir: pop=%v known=%v, want false,true", pop, known)
		}
	})
	t.Run("populated", func(t *testing.T) {
		fixtureRoot(t, "", "", true)
		pop, known := dataDirState()
		if !pop || !known {
			t.Errorf("populated dir: pop=%v known=%v, want true,true", pop, known)
		}
	})
}

func TestReadInitFile(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		fixtureRoot(t, "", `{"keys":["k1"],"root_token":"test-root-token-not-real"}`, true)
		shape, ok := readInitFile()
		if !ok || shape.RootToken != "test-root-token-not-real" {
			t.Errorf("ok=%v root=%q", ok, shape.RootToken)
		}
	})
	t.Run("absent", func(t *testing.T) {
		fixtureRoot(t, "", "", false)
		if _, ok := readInitFile(); ok {
			t.Error("absent init.json should be not-ok")
		}
	})
	t.Run("no-keys", func(t *testing.T) {
		fixtureRoot(t, "", `{"root_token":"x"}`, false)
		if _, ok := readInitFile(); ok {
			t.Error("init.json without unseal keys should be not-ok")
		}
	})
}

// --- disk-only verdict (openbao down) --------------------------------------

func TestDecideDisk(t *testing.T) {
	cases := []struct {
		name              string
		envToken, initJSON bool
		dPop, dKnown      bool
		want              Case
		wantOK            bool
	}{
		{"fresh", false, false, false, true, CaseFresh, true},
		{"orphaned-C3", true, false, false, true, CaseOrphanedToken, false},
		{"stale-json-C4", true, true, false, true, CaseStaleInitJSON, false},
		{"unknown-data-unreadable", true, false, false, false, CaseUnknown, false},
		{"unknown-token-and-data", false, true, false, true, CaseUnknown, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Result{EnvToken: c.envToken, InitJSON: c.initJSON, DataDir: c.dPop}
			got := r.decideDisk(c.dPop, c.dKnown)
			if got.Case != c.want || got.OK != c.wantOK {
				t.Errorf("Case=%v OK=%v, want %v %v", got.Case, got.OK, c.want, c.wantOK)
			}
			if !got.OK && got.Advice == "" {
				t.Error("non-OK verdict must carry advice")
			}
		})
	}
}

// --- API-authoritative verdict (openbao up) --------------------------------

func TestDecideReachable(t *testing.T) {
	shape := initFileShape{Keys: []string{"k1"}, RootToken: "test-root-token-not-real"}
	cases := []struct {
		name        string
		init, sealed, envTok, tokValid, initJSON bool
		dPop        bool
		want        Case
		wantOK      bool
	}{
		{"consistent-C2", true, false, true, true, true, true, CaseConsistent, true},
		{"lost-token-C5", true, false, false, false, true, true, CaseLostToken, false},
		{"wrong-token-C7", true, false, true, false, true, true, CaseWrongToken, false},
		{"stuck-sealed-C8", true, true, true, false, true, true, CaseStuckSealed, false},
		{"corrupt-C6", false, false, true, false, true, true, CaseCorrupt, false},
		{"orphaned-C3", false, false, true, false, false, false, CaseOrphanedToken, false},
		{"fresh-C1", false, false, false, false, false, false, CaseFresh, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Result{
				Initialized: c.init, Sealed: c.sealed,
				EnvToken: c.envTok, TokenValid: c.tokValid, InitJSON: c.initJSON,
			}
			got := r.decideReachable(shape, c.dPop)
			if got.Case != c.want || got.OK != c.wantOK {
				t.Errorf("Case=%v OK=%v, want %v %v", got.Case, got.OK, c.want, c.wantOK)
			}
		})
	}
}

// --- advice content ---------------------------------------------------------

func TestAdviceForMasksAndPaths(t *testing.T) {
	shape := initFileShape{Keys: []string{"k1"}, RootToken: "test-root-token-not-real"}
	adv := adviceFor(CaseLostToken, shape)
	if !strings.Contains(adv, "test-roo") {
		t.Errorf("C5 advice should show masked root_token, got: %s", adv)
	}
	if strings.Contains(adv, "test-root-token-not-real") {
		t.Error("advice must not contain the full unmasked token")
	}
	if !strings.Contains(adv, jsonPathHint) || !strings.Contains(adv, envPathHint) {
		t.Error("advice should name both the init.json and .env paths")
	}
	if adviceFor(CaseFresh, shape) != "" || adviceFor(CaseConsistent, shape) != "" {
		t.Error("OK cases should carry no advice")
	}
}
