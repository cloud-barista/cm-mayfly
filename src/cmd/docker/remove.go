package docker

import (
	"fmt"

	"github.com/cm-mayfly/cm-mayfly/src/common"
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

		if common.DockerFilePath == "" {
			fmt.Println("file is required")
		} else {
			var cmdStr string
			if volFlag && imgFlag {
				cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s down -v --rmi all", common.CMComposeProjectName, common.DockerFilePath)
			} else if volFlag {
				cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s down -v", common.CMComposeProjectName, common.DockerFilePath)
			} else if imgFlag {
				cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s down --rmi all", common.CMComposeProjectName, common.DockerFilePath)
			} else {
				cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s down", common.CMComposeProjectName, common.DockerFilePath)
			}

			//fmt.Println(cmdStr)
			common.SysCall(cmdStr)

			common.SysCallDockerComposePs()
		}

	},
}

var volFlag bool
var imgFlag bool

func init() {
	dockerCmd.AddCommand(removeCmd)

	pf := removeCmd.PersistentFlags()
	pf.StringVarP(&common.DockerFilePath, "file", "f", common.DefaultDockerComposeConfig, "User-defined configuration file")
	//	cobra.MarkFlagRequired(pf, "file")

	pf.BoolVarP(&volFlag, "volumes", "v", false, "Remove named volumes declared in the volumes section of the Compose file")
	pf.BoolVarP(&imgFlag, "images", "i", false, "Remove all images")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// removeCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// removeCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
