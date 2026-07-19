package tool

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// mysqlping runs as a compose healthcheck, so anything it logs is written
// repeatedly for the lifetime of the stack. The DSN must never carry the
// password into that log.
func TestMaskDSN(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no database",
			in:   "root:p4ssw0rd@tcp(db:3306)",
			want: "root:***@tcp(db:3306)",
		},
		{
			name: "with database",
			in:   "root:p4ssw0rd@tcp(db:3306)/airflow",
			want: "root:***@tcp(db:3306)/airflow",
		},
		{
			// A password containing '@' is why the split is on the last '@'
			// and not the first.
			name: "password contains at sign",
			in:   "root:p@ss@w0rd@tcp(db:3306)/airflow",
			want: "root:***@tcp(db:3306)/airflow",
		},
		{
			name: "empty password",
			in:   "root:@tcp(db:3306)",
			want: "root:***@tcp(db:3306)",
		},
		{
			name: "not a dsn is returned unchanged",
			in:   "garbage",
			want: "garbage",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := maskDSN(tc.in)
			if got != tc.want {
				t.Errorf("maskDSN(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if strings.Contains(got, "p4ssw0rd") || strings.Contains(got, "p@ss@w0rd") {
				t.Errorf("maskDSN leaked the password: %q", got)
			}
		})
	}
}

func TestMaskSecret(t *testing.T) {
	if got := maskSecret(""); got != "(empty)" {
		t.Errorf("maskSecret(\"\") = %q, want %q", got, "(empty)")
	}
	if got := maskSecret("p4ssw0rd"); got != "***" {
		t.Errorf("maskSecret(secret) = %q, want %q", got, "***")
	}
}

// newTestCmd builds a command carrying the same flag set as mysqlpingCmd,
// bound to the same package-level variables, so checkConfig can be exercised
// without running the real command.
func newTestCmd(t *testing.T) *cobra.Command {
	t.Helper()
	// Reset the shared state each time — these are package-level variables.
	user, password, database = "", "", ""
	host, port = "localhost", "3306"
	isVerbose = false

	c := &cobra.Command{Use: "mysqlping"}
	c.Flags().StringVarP(&user, "user", "u", "", "")
	c.Flags().StringVarP(&password, "password", "p", "", "")
	c.Flags().StringVarP(&host, "host", "", "localhost", "")
	c.Flags().StringVarP(&port, "port", "", "3306", "")
	c.Flags().StringVarP(&database, "database", "d", "", "")
	c.Flags().BoolVarP(&isVerbose, "verbose", "v", false, "")
	return c
}

// The help text promises that flags take precedence over the environment.
// The old checkConfig detected "flag was passed" by comparing against the
// default, so an explicit --host localhost looked identical to no flag at all
// and MYSQL_HOST silently won.
func TestFlagsBeatEnvironment(t *testing.T) {
	t.Setenv("MYSQL_USER", "envuser")
	t.Setenv("MYSQL_PASSWORD", "envpass")
	t.Setenv("MYSQL_HOST", "envhost")
	t.Setenv("MYSQL_PORT", "3307")
	t.Setenv("MYSQL_DATABASE", "envdb")

	c := newTestCmd(t)
	if err := c.ParseFlags([]string{
		"--user", "flaguser",
		"--password", "flagpass",
		"--host", "localhost", // deliberately equal to the default
		"--port", "3306", // deliberately equal to the default
		"--database", "flagdb",
	}); err != nil {
		t.Fatal(err)
	}

	if !checkConfig(c) {
		t.Fatal("checkConfig returned false with a complete configuration")
	}
	for _, tc := range []struct{ name, got, want string }{
		{"user", user, "flaguser"},
		{"password", password, "flagpass"},
		{"host", host, "localhost"},
		{"port", port, "3306"},
		{"database", database, "flagdb"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q (the flag must win over the environment)", tc.name, tc.got, tc.want)
		}
	}
}

// With no flags given, the environment still supplies every value — this is how
// the compose healthcheck is configured, so it must keep working.
func TestEnvironmentUsedWhenNoFlags(t *testing.T) {
	t.Setenv("MYSQL_USER", "envuser")
	t.Setenv("MYSQL_PASSWORD", "envpass")
	t.Setenv("MYSQL_HOST", "envhost")
	t.Setenv("MYSQL_PORT", "3307")
	t.Setenv("MYSQL_DATABASE", "envdb")

	c := newTestCmd(t)
	if err := c.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	if !checkConfig(c) {
		t.Fatal("checkConfig returned false with a complete environment")
	}
	for _, tc := range []struct{ name, got, want string }{
		{"user", user, "envuser"},
		{"password", password, "envpass"},
		{"host", host, "envhost"},
		{"port", port, "3307"},
		{"database", database, "envdb"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// Nothing configured anywhere: the defaults stay put and checkConfig reports
// the missing credentials rather than attempting a connection.
func TestMissingCredentialsRejected(t *testing.T) {
	t.Setenv("MYSQL_USER", "")
	t.Setenv("MYSQL_PASSWORD", "")
	t.Setenv("MYSQL_HOST", "")
	t.Setenv("MYSQL_PORT", "")
	t.Setenv("MYSQL_DATABASE", "")

	c := newTestCmd(t)
	if err := c.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	if checkConfig(c) {
		t.Error("checkConfig returned true without a user or password")
	}
	if host != "localhost" || port != "3306" {
		t.Errorf("defaults were disturbed: host=%q port=%q", host, port)
	}
}
