package docker

import (
	"fmt"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// pullCmd represents the pull command
var logCmd = &cobra.Command{
	Use:   "logs",
	Short: "View output from Cloud-Migrator system containers",
	Long:  `View output from Cloud-Migrator system containers`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[View output from Cloud-Migrator system containers.]")
		cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s logs --follow %s", ProjectName, DockerFilePath, ServiceName)
		//fmt.Println(cmdStr)
		common.SysCall(cmdStr)
	},
}

func init() {
	dockerCmd.AddCommand(logCmd)
}
