package docker

import (
	"fmt"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// stopAllFlag represents the --all flag for showing all containers including stopped ones
var stopAllFlag bool

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Cloud-Migrator System",
	Long:  `Stop Cloud-Migrator System`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Stop Cloud-Migrator]")
		fmt.Println()

		cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s stop %s", ProjectName, DockerFilePath, ServiceName)
		// // If there are additional arguments, treat them as services or additional commands and add them to the existing command with an additional
		// if len(args) > 0 {
		// 	cmdStr += args[0]

		// 	// Explicitly passing the service name as a filter (--service) option or argument would be fine.
		// 	// serviceName := args[0]
		// 	// cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s stop %s", ProjectName, DockerFilePath, serviceName)
		// }

		//fmt.Println(cmdStr)
		common.SysCall(cmdStr)

		SysCallDockerComposePsWithAll(stopAllFlag)
	},
}

func init() {
	dockerCmd.AddCommand(stopCmd)

	// Add --all flag to stop command
	stopCmd.Flags().BoolVarP(&stopAllFlag, "all", "a", false, "Show all containers including stopped ones")
}
