/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package k8s

import (
	"fmt"

	// "github.com/cm-mayfly/cm-mayfly/src/cmd"

	"github.com/cm-mayfly/cm-mayfly/src/cmd"
	"github.com/spf13/cobra"
)

var K8sprovider string

var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Installing and managing Cloud-Migrator's infrastructure",
	Long: `Build the environment of the infrastructure required for Cloud-Migrator and monitor the running status of the infrastructure.
For example, you can setup and run, stop, and ... Cloud-Migrator runtimes.

- ./mayfly k8s run [-f ../conf/k8s/cloud-migrator/values.yaml]
- ./mayfly k8s info
- ./mayfly k8s update [-f ../conf/k8s/cloud-migrator/values.yaml]
- ./mayfly k8s stop [-f ../conf/k8s/cloud-migrator/values.yaml]
- ./mayfly k8s remove [-f ../conf/k8s/cloud-migrator/values.yaml]
	     `,
	Run: func(cmd *cobra.Command, args []string) {
		//fmt.Println(cmd.UsageString())
		fmt.Println(cmd.Help())
	},
}

func init() {
	cmd.RootCmd.AddCommand(k8sCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// restCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// restCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
