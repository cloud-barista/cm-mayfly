package framework

import (
	"fmt"

	"github.com/cm-mayfly/cm-mayfly/src/common"
	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Cloud-Migrator System",
	Long:  `Stop Cloud-Migrator System`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Stop Cloud-Migrator]")
		fmt.Println()

		if common.FileStr == "" {
			fmt.Println("file is required")
		} else {
			common.FileStr = common.GenConfigPath(common.FileStr, common.CMMayflyMode)
			var cmdStr string
			switch common.CMMayflyMode {
			case common.ModeDockerCompose:
				cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s stop", common.CMComposeProjectName, common.FileStr)
				//fmt.Println(cmdStr)
				common.SysCall(cmdStr)

				common.SysCallDockerComposePs()
			case common.ModeKubernetes:
				cmdStr = fmt.Sprintf("helm uninstall --namespace %s %s", common.CMK8sNamespace, common.CMHelmReleaseName)
				common.SysCall(cmdStr)
			default:

			}
		}

	},
}

func init() {
	dockerCmd.AddCommand(stopCmd)

	pf := stopCmd.PersistentFlags()
	pf.StringVarP(&common.FileStr, "file", "f", common.NotDefined, "User-defined configuration file")
	//	cobra.MarkFlagRequired(pf, "file")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// stopCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// stopCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
