package docker

import (
	"fmt"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Setup and Run Cloud-Migrator System",
	Long:  `Setup and Run Cloud-Migrator System`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Setup and Run Cloud-Migrator]")
		fmt.Println()

		//cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s up -d", ProjectName, DockerFilePath)
		cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s up ", ProjectName, DockerFilePath)

		// If there are additional arguments, treat them as services or additional commands and add them to the existing command with an additional
		if len(args) > 0 {
			cmdStr += args[0]

			// Explicitly passing the service name as a filter (--service) option or argument would be fine.
			// serviceName := args[0]
			// cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s up %s", ProjectName, DockerFilePath, serviceName)
		}

		//fmt.Println(cmdStr)
		common.SysCall(cmdStr)
	},
}

func init() {
	dockerCmd.AddCommand(runCmd)
}
