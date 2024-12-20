package k8s

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

		if K8sFilePath == "" {
			fmt.Println("file is required")
		} else {
			var cmdStr string

			cmdStr = fmt.Sprintf("helm uninstall --namespace %s %s", CMK8sNamespace, CMHelmReleaseName)
			common.SysCall(cmdStr)

			cmdStr = fmt.Sprintf("kubectl delete ns %s", CMK8sNamespace)
			common.SysCall(cmdStr)
		}

	},
}

var volFlag bool
var imgFlag bool

func init() {
	k8sCmd.AddCommand(removeCmd)

	pf := removeCmd.PersistentFlags()
	pf.StringVarP(&K8sFilePath, "file", "f", DefaultKubernetesConfig, "User-defined configuration file")
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
