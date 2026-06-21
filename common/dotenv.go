package common

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ParseDotEnv reads a docker-compose .env file and returns a key→value map.
// It supports KEY=value lines (with optional surrounding double or single quotes
// on the value), ignores blank lines and comments, and tolerates `export
// KEY=value` shell-style prefixes. Unknown line shapes are skipped rather than
// erroring out — docker compose's own parser is the source of truth for full
// syntax, this routine only needs to surface values. Shared by `mayfly infra`
// (.env validation) and `mayfly api`/`rest` (auth env resolution).
func ParseDotEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	values := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if len(val) >= 2 {
			first, last := val[0], val[len(val)-1]
			if (first == '"' || first == '\'') && first == last {
				val = val[1 : len(val)-1]
			}
		}
		values[key] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

// DefaultDotEnvPath returns the conf/docker/.env path (next to the default
// docker-compose file). This is the single shared environment file.
func DefaultDotEnvPath() string {
	return filepath.Join(filepath.Dir(DefaultDockerComposeConfig), ".env")
}

var (
	dotEnvOnce  sync.Once
	dotEnvCache map[string]string
)

// lookupDotEnv returns the value of name from conf/docker/.env (parsed once per
// process), or "" if the file is missing or the key is absent.
func lookupDotEnv(name string) string {
	dotEnvOnce.Do(func() {
		if m, err := ParseDotEnv(DefaultDotEnvPath()); err == nil {
			dotEnvCache = m
		} else {
			dotEnvCache = map[string]string{}
		}
	})
	return dotEnvCache[name]
}

// envRefName returns the VAR name if s is exactly "${VAR}" (trimmed), else
// ("", false).
func envRefName(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		name := strings.TrimSpace(s[2 : len(s)-1])
		if name != "" {
			return name, true
		}
	}
	return "", false
}

// ResolveEnvRef expands a "${VAR}" reference found in an api.yaml auth value.
// Resolution priority: process environment (os.Getenv) → conf/docker/.env file.
// A non-reference literal is returned unchanged. The second return value is true
// only when s WAS a ${VAR} reference that resolved to no value anywhere — so the
// caller can warn and avoid sending a silent default. There is intentionally no
// implicit "default" fallback.
func ResolveEnvRef(s string) (value string, refUnset bool) {
	name, ok := envRefName(s)
	if !ok {
		return s, false
	}
	if v := os.Getenv(name); v != "" {
		return v, false
	}
	if v := lookupDotEnv(name); v != "" {
		return v, false
	}
	return "", true
}
