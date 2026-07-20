package docker

import (
	"fmt"

	"github.com/spf13/cobra"
)

// pullCmd represents the pull command
var pullCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the Docker images of the subsystems that make up the Cloud-Migrator system.",
	Long:  `Install the Docker images of the subsystems that make up the Cloud-Migrator system.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Install the Docker images of the subsystems that make up the Cloud-Migrator system.]")
		fmt.Println()

		services, err := resolveServices(ServiceName)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			return
		}

		if err := runCompose(append([]string{"pull"}, services...)...); err != nil {
			fmt.Printf("❌ docker compose pull failed: %v\n", err)
		}
	},
}

func init() {
	dockerCmd.AddCommand(pullCmd)
}
