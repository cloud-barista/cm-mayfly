package docker

import (
	"fmt"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// pullCmd represents the pull command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the latest Docker images of the subsystems that make up the Cloud-Migrator system.",
	Long:  `Update the latest Docker images of the subsystems that make up the Cloud-Migrator system.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Update the Docker images of the subsystems that make up the Cloud-Migrator system.]")
		//============================
		// Pull
		//============================
		fmt.Println("[Install the latest Docker images of the Cloud-Migrator subsystems.]")
		fmt.Println()

		cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s pull %s", ProjectName, DockerFilePath, ServiceName)
		//fmt.Println(cmdStr)
		common.SysCall(cmdStr)

		//============================
		// RUN
		//============================
		fmt.Println("\n\n[Restart based on the installed latest Docker images.]")
		fmt.Println()

		detachModeOption := ""
		if DetachMode {
			detachModeOption = "-d"
		}
		cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s up %s %s", ProjectName, DockerFilePath, detachModeOption, ServiceName)

		//fmt.Println(cmdStr)
		common.SysCall(cmdStr)

	},
}

func init() {
	dockerCmd.AddCommand(updateCmd)
}
