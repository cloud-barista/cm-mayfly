/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package setup

import (
	//"github.com/cm-mayfly/cm-mayfly/cmd" //임시로 주석처리
	//"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Support for Additional Tasks After Container Setup",
	Long: `Supports installation tasks for specific containers after setting up the Cloud-migrator's infrastructure.
	     `,
	Run: func(cmd *cobra.Command, args []string) {
		//fmt.Println(cmd.UsageString())
		//fmt.Println(cmd.Help())
		cmd.Help()
	},
}

func init() {
	//cmd.RootCmd.AddCommand(setupCmd)
}
