package docker

import (
	"fmt"

	"github.com/spf13/cobra"
)

// stopAllFlag represents the --all flag for showing all containers including stopped ones
var stopAllFlag bool

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Cloud-Migrator System",
	Long:  `Stop Cloud-Migrator System`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Stop Cloud-Migrator]")
		fmt.Println()

		services, err := resolveServices(ServiceName)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			return
		}

		if err := runCompose(append([]string{"stop"}, services...)...); err != nil {
			fmt.Printf("❌ docker compose stop failed: %v\n", err)
			return
		}

		SysCallDockerComposePsWithAll(stopAllFlag)
	},
}

func init() {
	dockerCmd.AddCommand(stopCmd)

	// Add --all flag to stop command
	stopCmd.Flags().BoolVarP(&stopAllFlag, "all", "a", false, "Show all containers including stopped ones")
}
