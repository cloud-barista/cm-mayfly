package k8s

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

		if common.K8sFilePath == "" {
			fmt.Println("file is required")
		} else {
			var cmdStr string

			cmdStr = fmt.Sprintf("helm uninstall --namespace %s %s", common.CMK8sNamespace, common.CMHelmReleaseName)
			common.SysCall(cmdStr)

		}

	},
}

func init() {
	k8sCmd.AddCommand(stopCmd)

	pf := stopCmd.PersistentFlags()
	pf.StringVarP(&common.K8sFilePath, "file", "f", common.DefaultKubernetesConfig, "User-defined configuration file")
	//	cobra.MarkFlagRequired(pf, "file")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// stopCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// stopCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
