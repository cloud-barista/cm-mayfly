package docker

import (
	"fmt"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// pullCmd represents the pull command
var pullCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the Docker images of the subsystems that make up the Cloud-Migrator system.",
	Long:  `Install the Docker images of the subsystems that make up the Cloud-Migrator system.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Install the Docker images of the subsystems that make up the Cloud-Migrator system.]")
		fmt.Println()

		cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s pull %s", ProjectName, DockerFilePath, ServiceName)
		//fmt.Println(cmdStr)
		common.SysCall(cmdStr)
	},
}

func init() {
	dockerCmd.AddCommand(pullCmd)
}
