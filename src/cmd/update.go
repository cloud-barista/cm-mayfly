package cmd

import (
	"fmt"
	"strings"

	"github.com/cm-mayfly/cm-mayfly/src/common"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:     "update",
	Aliases: []string{"apply"},
	Short:   "Update Cloud-Migrator System",
	Long:    `Update Cloud-Migrator System`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Update Cloud-Migrator]")
		fmt.Println()

		if common.FileStr == "" {
			fmt.Println("file is required")
		} else {
			common.FileStr = common.GenConfigPath(common.FileStr, common.CMMayflyMode)

			var cmdStr string
			switch common.CMMayflyMode {
			case common.ModeDockerCompose:
				fmt.Println("cm-mayfly Docker Compose mode does not support 'update/apply' subcommand.")

			case common.ModeKubernetes:
				cmdStr = fmt.Sprintf("helm upgrade --namespace %s --install %s -f %s ../helm-chart", common.CMK8sNamespace, common.CMHelmReleaseName, common.FileStr)
				if strings.ToLower(k8sprovider) == "gke" || strings.ToLower(k8sprovider) == "aks" {
					cmdStr += " --set metricServer.enabled=false"
				}
				//fmt.Println(cmdStr)
				common.SysCall(cmdStr)
			default:

			}

		}

	},
}

func init() {
	//rootCmd.AddCommand(updateCmd)

	pf := updateCmd.PersistentFlags()
	pf.StringVarP(&common.FileStr, "file", "f", common.NotDefined, "User-defined configuration file")
	pf.StringVarP(&k8sprovider, "k8sprovider", "", common.NotDefined, "Kind of Managed K8s services")

	//	cobra.MarkFlagRequired(pf, "file")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// updateCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// updateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
