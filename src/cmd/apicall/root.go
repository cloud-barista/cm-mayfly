/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package rest

import (
	"fmt"

	"github.com/cm-mayfly/cm-mayfly/src/cmd"
	"github.com/spf13/cobra"
)

// svcCmd represents the svc command
var svcCmd = &cobra.Command{
	Use:   "svc",
	Short: "Open API calls to the Cloud-Migrator system",
	Long: `Open API calls to the Cloud-Migrator system. For example:

Login , ...`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("svc called")
	},
}

func init() {
	cmd.RootCmd.AddCommand(svcCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// svcCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// svcCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
