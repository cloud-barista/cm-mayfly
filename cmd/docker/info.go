package docker

import (
	"fmt"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// infoAllFlag represents the --all flag for showing all containers including stopped ones
var infoAllFlag bool

// infoCmd represents the info command.
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get information of Cloud-Migrator System",
	Long:  `Get information of Cloud-Migrator System. Information about containers and container images`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Get info for Cloud-Migrator runtimes]")
		fmt.Println()
		SysCallDockerComposePsWithAll(infoAllFlag)
		fmt.Println("")

		fmt.Println("[v]Status of Cloud-Migrator runtime images")
		cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s images %s", ProjectName, DockerFilePath, ServiceName)
		//fmt.Println(cmdStr)
		common.SysCall(cmdStr)
	},
}

func init() {
	dockerCmd.AddCommand(infoCmd)

	// Add --all flag to info command
	infoCmd.Flags().BoolVarP(&infoAllFlag, "all", "a", false, "Show all containers including stopped ones")
}
