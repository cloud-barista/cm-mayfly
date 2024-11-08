package k8s

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Setup and Run Cloud-Migrator System",
	Long:  `Setup and Run Cloud-Migrator System`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Setup and Run Cloud-Migrator]")
		fmt.Println()

		if K8sFilePath == "" {
			fmt.Println("--file (-f) argument is required but not provided.")
		} else {
			var cmdStr string
			if K8sprovider == common.NotDefined {
				fmt.Print(`--k8sprovider argument is required but not provided.
					e.g.
					--k8sprovider=gke
					--k8sprovider=eks
					--k8sprovider=aks
					--k8sprovider=mcks
					--k8sprovider=minikube
					--k8sprovider=kubeadm

					`)

				return
			}

			helmChartPath := filepath.Dir(K8sFilePath)

			cmdStr = fmt.Sprintf("kubectl create ns %s --dry-run=client -o yaml | kubectl apply -f -", CMK8sNamespace)
			common.SysCall(cmdStr)

			if strings.ToLower(K8sprovider) == "gke" || strings.ToLower(K8sprovider) == "eks" || strings.ToLower(K8sprovider) == "aks" {
				cmdStr = fmt.Sprintf("helm install --namespace %s %s -f %s %s", CMK8sNamespace, CMHelmReleaseName, K8sFilePath, helmChartPath)
				cmdStr += " --set cb-restapigw.service.type=LoadBalancer"
				cmdStr += " --set cb-webtool.service.type=LoadBalancer"

				if strings.ToLower(K8sprovider) == "gke" || strings.ToLower(K8sprovider) == "aks" {
					cmdStr += " --set metricServer.enabled=false"
				}

				common.SysCall(cmdStr)
			} else {
				cmdStr = fmt.Sprintf("helm install --namespace %s %s -f %s %s", CMK8sNamespace, CMHelmReleaseName, K8sFilePath, helmChartPath)
				common.SysCall(cmdStr)
			}
		}

	},
}

func init() {
	k8sCmd.AddCommand(runCmd)

	pf := runCmd.PersistentFlags()
	pf.StringVarP(&K8sFilePath, "file", "f", DefaultKubernetesConfig, "User-defined configuration file")
	pf.StringVarP(&K8sprovider, "k8sprovider", "k", common.NotDefined, "Kind of Managed K8s services")

	// runCmd.MarkPersistentFlagRequired("k8sprovider")

	//	cobra.MarkFlagRequired(pf, "file")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// runCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// runCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
