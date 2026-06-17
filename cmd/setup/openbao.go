// Package setup — `setup openbao` subcommands.
//
// These are thin wrappers around internal/openbao. Keeping the logic in the
// internal package lets `infra run` reuse the same code path so behavior stays
// consistent (single source of truth).
package setup

import (
	"fmt"
	"os"

	"github.com/cm-mayfly/cm-mayfly/internal/openbao"
	"github.com/spf13/cobra"
)

var openbaoCmd = &cobra.Command{
	Use:   "openbao",
	Short: "Manage the OpenBao secret manager (init / unseal / status)",
	Long: `Manage the OpenBao container that backs cb-tumblebug's encrypted credential store.

Standard first-run flow:
  1. ./mayfly setup openbao init      (one-time; writes VAULT_TOKEN to .env)
  2. ./mayfly infra run               (brings up everything else)
  3. ./mayfly setup tumblebug-init    (registers CSPs, fetches catalog)

'mayfly infra run' also calls 'openbao init' automatically when VAULT_TOKEN is
missing in .env, so step 1 is mainly there for users who prefer explicit control
or for the --force re-initialization path.`,
	Run: func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
}

var openbaoForce bool

var openbaoInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize OpenBao (one-time setup; writes VAULT_TOKEN to .env)",
	Long: `Bring up the OpenBao container alone, initialize it, and write the
resulting VAULT_TOKEN into conf/docker/.env so that cb-tumblebug and
mc-terrarium pick it up when they start.

By default this command refuses to run if .env already has a non-empty
VAULT_TOKEN — re-initializing OpenBao would generate a new unseal key + root
token and the existing encrypted data would become inaccessible. Use --force
only after wiping the openbao data volume (docker compose down -v).`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := openbao.Init(openbaoForce); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	},
}

var openbaoUnsealCmd = &cobra.Command{
	Use:   "unseal",
	Short: "Unseal OpenBao using the persisted unseal key (manual path)",
	Long: `Read the first unseal key from
conf/docker/data/openbao/secrets/openbao-init.json and apply it to OpenBao.

Normally the openbao-unseal sidecar does this automatically on every container
start, so this command is only needed when the sidecar is intentionally
disabled (e.g. while trialing KMS auto-unseal, or in a manual ops mode).`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := openbao.Unseal(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	},
}

var openbaoStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show OpenBao + container VAULT_TOKEN consistency",
	Long: `Print a one-screen summary of:
  - OpenBao API reachability / initialization / seal state
  - .env VAULT_TOKEN (masked)
  - cb-tumblebug and mc-terrarium container VAULT_TOKEN presence

Notes section flags common drift patterns and suggests the matching command
(e.g. "container token empty although .env has one — recreate it with ...").`,
	Run: func(cmd *cobra.Command, args []string) {
		s := openbao.Status()
		fmt.Println("\n[OpenBao status]")
		fmt.Printf("  OpenBao API     : reachable=%v initialized=%v sealed=%v\n",
			s.OpenbaoReachable, s.OpenbaoInitialized, s.OpenbaoSealed)
		fmt.Printf("  .env VAULT_TOKEN: %s\n", s.EnvTokenMasked)
		fmt.Printf("  cb-tumblebug    : VAULT_TOKEN %s\n", yesNo(s.TumblebugTokenSet, "set", "(empty)"))
		fmt.Printf("  mc-terrarium    : VAULT_TOKEN %s\n", yesNo(s.TerrariumTokenSet, "set", "(empty)"))
		if len(s.Notes) > 0 {
			fmt.Println("\nNotes:")
			for _, n := range s.Notes {
				fmt.Println("  - " + n)
			}
		}
	},
}

func yesNo(b bool, t, f string) string {
	if b {
		return t
	}
	return f
}

func init() {
	openbaoInitCmd.Flags().BoolVar(&openbaoForce, "force", false,
		"Re-initialize even if VAULT_TOKEN is already set (existing encrypted data will be lost)")
	openbaoCmd.AddCommand(openbaoInitCmd)
	openbaoCmd.AddCommand(openbaoUnsealCmd)
	openbaoCmd.AddCommand(openbaoStatusCmd)
	setupCmd.AddCommand(openbaoCmd)
}
