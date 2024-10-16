package docker

import (
	"fmt"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Install and Run Cloud-Migrator System",
	Long:  `Install and Run Cloud-Migrator System`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Install and Run Cloud-Migrator System]")
		fmt.Println()

		detachModeOption := ""
		if DetachMode {
			detachModeOption = "-d"
		}
		cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s up %s %s", ProjectName, DockerFilePath, detachModeOption, ServiceName)

		// // If there are additional arguments, treat them as services or additional commands and add them to the existing command with an additional
		// if len(args) > 0 {
		// 	cmdStr += args[0]

		// 	// Explicitly passing the service name as a filter (--service) option or argument would be fine.
		// 	// serviceName := args[0]
		// 	// cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s up %s", ProjectName, DockerFilePath, serviceName)
		// }

		//fmt.Println(cmdStr)
		common.SysCall(cmdStr)
	},
}

var DetachMode bool

func init() {
	dockerCmd.AddCommand(runCmd)

	// background mode
	runCmd.Flags().BoolVarP(&DetachMode, "detach", "d", false, "Detached mode: Run containers in the background")

	// // ServiceName is used when you want to specify only a specific service
	// runCmd.Flags().StringVarP(&ServiceName, "service", "s", "", "Want to target only one specific service(Default : all)")
}
