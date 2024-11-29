package tool

import (

	//"github.com/cm-mayfly/cm-mayfly/cmd" //임시로 주석처리
	//"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/cm-mayfly/cm-mayfly/cmd"
	"github.com/spf13/cobra"
)

var toolCmd = &cobra.Command{
	Use:   "tool",
	Short: "Provides additional functions for managing Docker Compose or the Cloud-Migrator system.",
	Long: `Provides additional functions for managing Docker Compose or the Cloud-Migrator system.
	     `,
	Run: func(cmd *cobra.Command, args []string) {
		//fmt.Println(cmd.UsageString())
		//fmt.Println(cmd.Help())
		cmd.Help()
	},
}

func init() {
	cmd.RootCmd.AddCommand(toolCmd)
}
