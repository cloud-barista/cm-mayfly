package docker

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	tailLines  string
	sinceTime  string
	followLogs bool
	noFollow   bool
)

// logCmd represents the logs command
var logCmd = &cobra.Command{
	Use:   "logs",
	Short: "View output from Cloud-Migrator system containers",
	Long: `View output from Cloud-Migrator system containers with flexible options.

Examples:
  mayfly infra logs                    # Show last 10 lines and follow (all services)
  mayfly infra logs --no-follow        # Show last 10 lines and exit (all services)
  mayfly infra logs -s cb-tumblebug    # Show logs for specific service (cb-tumblebug) and follow
  mayfly infra logs --tail 50          # Show last 50 lines and follow
  mayfly infra logs --tail all         # Show all logs from beginning and follow
  mayfly infra logs --since 1h         # Show logs from last 1 hour and follow
  mayfly infra logs --since 1h --tail all  # Show all logs from last 1 hour and follow
  mayfly infra logs -s cb-tumblebug --tail 50  # Show last 50 lines for cb-tumblebug service and follow
  mayfly infra logs -s cm-ant --tail 20 --since 1h  # Show last 20 lines for cm-ant from last 1 hour and follow
  mayfly infra logs --tail 20 --since 30m --no-follow  # Show last 20 lines from last 30 minutes and exit

Note:
  --since supports RFC3339 timestamps (e.g. 2024-01-02T13:23:37Z) or relative time (e.g. 42m, 1h, 2h30m)
  Timestamps with spaces are NOT supported. Use 'T' separator instead of space.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[View output from Cloud-Migrator system containers.]")

		services, err := resolveSelectedServices()
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			return
		}

		// Build the docker compose logs argument vector. Flag values reach docker
		// as single arguments, so a value carrying shell metacharacters is passed
		// through literally rather than interpreted.
		logArgs := []string{"logs"}

		// Add --follow option (default: true, can be disabled with --no-follow)
		if followLogs && !noFollow {
			logArgs = append(logArgs, "--follow")
		}

		// Add --tail option (default: last 10 lines)
		if tailLines != "" {
			logArgs = append(logArgs, "--tail", tailLines)
		} else {
			logArgs = append(logArgs, "--tail", "10")
		}

		// Add --since option if specified
		if sinceTime != "" {
			logArgs = append(logArgs, "--since", sinceTime)
		}

		logArgs = append(logArgs, services...)

		if err := runCompose(logArgs...); err != nil {
			fmt.Printf("❌ docker compose logs failed: %v\n", err)
		}
	},
}

func init() {
	dockerCmd.AddCommand(logCmd)

	// Add flags for log options
	logCmd.Flags().StringVarP(&tailLines, "tail", "n", "", "Number of lines to show from the end of the logs (default: 10, use 'all' for all logs)")
	logCmd.Flags().StringVar(&sinceTime, "since", "", "Show logs since timestamp (e.g. 2024-01-02T13:23:37Z) or relative (e.g. 42m, 1h)")
	logCmd.Flags().BoolVar(&followLogs, "follow", true, "Follow log output (similar to tail -f)")
	logCmd.Flags().BoolVar(&noFollow, "no-follow", false, "Disable follow mode (show logs and exit)")
}
