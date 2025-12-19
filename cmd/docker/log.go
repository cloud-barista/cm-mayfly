package docker

import (
	"fmt"

	"github.com/cm-mayfly/cm-mayfly/common"
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

		// Build docker compose logs command
		cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s logs", ProjectName, DockerFilePath)

		// Add --follow option (default: true, can be disabled with --no-follow)
		if followLogs && !noFollow {
			cmdStr += " --follow"
		}

		// Add --tail option
		if tailLines != "" {
			cmdStr += fmt.Sprintf(" --tail %s", tailLines)
		} else {
			// Default: show last 10 lines
			cmdStr += " --tail 10"
		}

		// Add --since option if specified (quote the value to handle special characters)
		if sinceTime != "" {
			cmdStr += fmt.Sprintf(" --since '%s'", sinceTime)
		}

		// Add service name if specified
		if ServiceName != "" {
			cmdStr += " " + ServiceName
		}

		//fmt.Println(cmdStr)
		common.SysCall(cmdStr)
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
