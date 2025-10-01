package docker

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Stop and Remove Cloud-Migrator System or specific services",
	Long: `Stop and Remove Cloud-Migrator System or specific services.

For entire system removal:
  - Removes all containers, networks, and optionally images/volumes
  - Use --all flag to remove everything including orphaned containers

For specific service removal:
  - Removes only the specified service container
  - Preserves Docker networks to prevent connectivity issues
  - Use -s flag to specify target service(s)`,
	Run: func(cmd *cobra.Command, args []string) {

		fmt.Println("\n[Remove Cloud-Migrator]")
		fmt.Println()

		var cmdStr string
		var removeOptions string
		if volFlag && imgFlag || allFlag {
			removeOptions = "--volumes --rmi all"
		} else if volFlag {
			removeOptions = "--volumes"
		} else if imgFlag {
			removeOptions = "--rmi all"
		} else {
			removeOptions = ""
		}

		// Display removal target information
		fmt.Println("Removal Target:")

		// Display service target
		if ServiceName == "" {
			fmt.Println("  Services: All services")
		} else {
			fmt.Printf("  Services: %s\n", ServiceName)
		}

		// Display removal scope
		if allFlag {
			fmt.Println("  Scope: Containers + Images + Volumes + Orphaned Containers (all)")
		} else if volFlag && imgFlag {
			fmt.Println("  Scope: Containers + Images + Volumes")
		} else if volFlag {
			fmt.Println("  Scope: Containers + Volumes")
		} else if imgFlag {
			fmt.Println("  Scope: Containers + Images")
		} else {
			fmt.Println("  Scope: Containers only (images preserved)")
		}
		fmt.Println()

		// Display additional options information
		if !imgFlag && !volFlag && !allFlag {
			fmt.Println("Additional Options:")
			fmt.Println("  -i, --images    : Also remove images")
			fmt.Println("  -v, --volumes   : Also remove named volumes (local mounts preserved)")
			fmt.Println("  --all           : Remove everything (images + volumes + orphaned containers)")
			fmt.Println()
		} else if volFlag && !allFlag {
			fmt.Println("Note: Named volumes will be removed, but local mount volumes are preserved.")
			fmt.Println()
		}

		// Request user confirmation
		fmt.Print("Do you want to proceed with the removal? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Println("Removal cancelled.")
			return
		}

		convertedServiceName := convertServiceNameForDockerCompose(ServiceName)

		if ServiceName == "" {
			// Remove entire system
			// Add --remove-orphans to clean up orphaned containers and networks
			if removeOptions != "" {
				cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s down %s --remove-orphans", ProjectName, DockerFilePath, removeOptions)
			} else {
				cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s down --remove-orphans", ProjectName, DockerFilePath)
			}
		} else {
			// Remove specific service only
			// 1. First stop the service
			stopCmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s stop %s", ProjectName, DockerFilePath, convertedServiceName)
			common.SysCall(stopCmdStr)

			// 2. Remove service (apply image/volume options)
			if volFlag && imgFlag || allFlag {
				// Remove volumes and images together
				cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s rm -f -v %s", ProjectName, DockerFilePath, convertedServiceName)
				common.SysCall(cmdStr)
				// Image removal (direct removal using docker images command)
				// Note: Removing images for specific services is complex,
				// so it's safer to guide users to manual removal
				fmt.Printf("⚠️  Note: Image removal for specific services is complex.\n")
				fmt.Printf("   To remove images manually, use: docker images | grep %s\n", convertedServiceName)
			} else if volFlag {
				// Remove volumes only
				cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s rm -f -v %s", ProjectName, DockerFilePath, convertedServiceName)
			} else if imgFlag {
				// Remove images only
				cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s rm -f %s", ProjectName, DockerFilePath, convertedServiceName)
				common.SysCall(cmdStr)
				// Image removal guidance
				fmt.Printf("⚠️  Note: Image removal for specific services is complex.\n")
				fmt.Printf("   To remove images manually, use: docker images | grep %s\n", convertedServiceName)
			} else {
				// Remove containers only
				cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s rm -f %s", ProjectName, DockerFilePath, convertedServiceName)
			}
		}

		//fmt.Println(cmdStr)
		common.SysCall(cmdStr)

		SysCallDockerComposePsWithAll(false)
	},
}

var allFlag bool
var volFlag bool
var imgFlag bool

func init() {
	dockerCmd.AddCommand(removeCmd)

	pf := removeCmd.PersistentFlags()
	pf.BoolVarP(&allFlag, "all", "a", false, "Remove all images, volumes, networks, and orphaned containers")
	pf.BoolVarP(&volFlag, "volumes", "v", false, "Remove named volumes declared in the volumes section of the Compose file")
	pf.BoolVarP(&imgFlag, "images", "i", false, "Remove all images")
}
