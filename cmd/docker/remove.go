package docker

import (
	"fmt"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Stop and Remove Cloud-Migrator System",
	Long:  `Stop and Remove Cloud-Migrator System. Stop and Remove Cloud-Migrator runtimes and related container images and meta-DB if necessary`,
	Run: func(cmd *cobra.Command, args []string) {

		fmt.Println("\n[Remove Cloud-Migrator]")
		fmt.Println()

		var cmdStr string
		var removeOptions string
		if volFlag && imgFlag || allFlag {
			removeOptions = "--volumes --rmi all"
		} else if volFlag {
			removeOptions = "--volumes"
		} else if imgFlag {
			removeOptions = "--rmi all"
		} else {
			removeOptions = ""
		}

		cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s down %s %s", ProjectName, DockerFilePath, removeOptions, ServiceName)

		//fmt.Println(cmdStr)
		common.SysCall(cmdStr)

		SysCallDockerComposePs()
	},
}

var allFlag bool
var volFlag bool
var imgFlag bool

func init() {
	dockerCmd.AddCommand(removeCmd)

	pf := removeCmd.PersistentFlags()
	pf.BoolVarP(&allFlag, "all", "", false, "Remove all images and volumes and networks")
	pf.BoolVarP(&volFlag, "volumes", "v", false, "Remove named volumes declared in the volumes section of the Compose file")
	pf.BoolVarP(&imgFlag, "images", "i", false, "Remove all images")
}
