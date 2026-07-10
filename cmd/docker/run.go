package docker

import (
	"fmt"
	"os"
	"strings"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/cm-mayfly/cm-mayfly/internal/openbao"
	"github.com/spf13/cobra"
)

// showServiceInfo displays service information grouped by category
func showServiceInfo(services map[string]ServiceInfo) {
	fmt.Println("🚀 Services to be started:")
	fmt.Println()

	// Group services by category
	categories := make(map[string][]ServiceInfo)
	for _, service := range services {
		categories[service.Category] = append(categories[service.Category], service)
	}

	// Define category order and icons
	categoryOrder := []struct {
		name string
		icon string
	}{
		{"Core Services", "🎯"},
		{"Database", "🗄️"},
		{"Cache/Storage", "💾"},
		{"Workflow Engine", "⚙️"},
		{"Dependencies", "🔧"},
	}

	totalServices := len(services)
	fmt.Printf("📊 Total Services: %d\n\n", totalServices)

	// Display services by category
	for _, catInfo := range categoryOrder {
		if services, exists := categories[catInfo.name]; exists {
			fmt.Printf("%s %s (%d services)\n", catInfo.icon, catInfo.name, len(services))

			// Find the longest service name and image strings for proper table width
			maxServiceLen := 15 // minimum width
			maxImageLen := 25   // minimum width for image columns

			for _, service := range services {
				if len(service.Name) > maxServiceLen {
					maxServiceLen = len(service.Name)
				}
				if len(service.Image) > maxImageLen {
					maxImageLen = len(service.Image)
				}
			}

			// Create table header with dynamic width
			headerFormat := fmt.Sprintf("┌─%%-%ds─┬─%%-%ds─┐\n", maxServiceLen, maxImageLen)
			separatorFormat := fmt.Sprintf("├─%%-%ds─┼─%%-%ds─┤\n", maxServiceLen, maxImageLen)
			footerFormat := fmt.Sprintf("└─%%-%ds─┴─%%-%ds─┘\n", maxServiceLen, maxImageLen)

			// Print top border
			fmt.Printf(headerFormat, strings.Repeat("─", maxServiceLen), strings.Repeat("─", maxImageLen))
			// Print header row
			fmt.Printf("│ %-*s │ %-*s │\n", maxServiceLen, "Service", maxImageLen, "Image")
			// Print separator
			fmt.Printf(separatorFormat, strings.Repeat("─", maxServiceLen), strings.Repeat("─", maxImageLen))

			for _, service := range services {
				fmt.Printf("│ %-*s │ %-*s │\n", maxServiceLen, service.Name, maxImageLen, service.Image)
			}

			fmt.Printf(footerFormat, strings.Repeat("─", maxServiceLen), strings.Repeat("─", maxImageLen))
			fmt.Println()
		}
	}
}

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Install and Run Cloud-Migrator System",
	Long:  `Install and Run Cloud-Migrator System`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Install and Run Cloud-Migrator System]")
		fmt.Println()

		// Parse docker-compose.yaml to show service information
		services, err := parseDockerComposeImages()
		if err != nil {
			fmt.Printf("⚠️ Failed to parse docker-compose.yaml: %v\n", err)
			fmt.Printf("🔄 Proceeding with regular run...\n")
		} else {
			// If specific service is requested, only show that service
			if ServiceName != "" {
				if serviceInfo, exists := services[ServiceName]; exists {
					services = map[string]ServiceInfo{ServiceName: serviceInfo}
				} else {
					fmt.Printf("⚠️ Service %s not found in docker-compose.yaml\n", ServiceName)
					return
				}
			}

			// Show service information
			showServiceInfo(services)
		}

		// Ask user for confirmation before proceeding with installation
		fmt.Print("\nDo you want to proceed with the installation? (y/N): ")
		var response string
		fmt.Scanln(&response)

		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("❌ Installation cancelled by user")
			return
		}

		// Mirror the upstream cb-tumblebug `make up` staged flow: bring up
		// openbao alone first, initialize it (which writes VAULT_TOKEN into
		// .env), then bring up the rest with the token already populated.
		// Without this step, cb-tumblebug / mc-terrarium would start with an
		// empty VAULT_TOKEN frozen into their environment and a `docker
		// compose restart` after a sidecar init wouldn't re-evaluate .env —
		// that's the regression we hit on staging. Only the
		// full-stack default path needs this; a targeted `-s` run is left
		// untouched.
		//
		// A shared state-consistency preflight decides what to do:
		// it may start openbao alone to get an authoritative reading. C1 fresh
		// → auto-init; C2 consistent → skip; any inconsistency (stale token,
		// wiped storage, invalid token, …) → print the specific remediation and
		// stop BEFORE starting the rest, so the stack never deadlocks in the
		// half-up "Created" state a broken OpenBao would otherwise leave behind.
		if ServiceName == "" {
			pf := openbao.Preflight(true) // run may start openbao alone to diagnose
			switch {
			case pf.Case == openbao.CaseFresh:
				fmt.Println()
				fmt.Println("ℹ VAULT_TOKEN not found in .env — initializing OpenBao first")
				fmt.Println("  (mirrors cb-tumblebug `make up` staged flow: openbao alone, then init, then the rest).")
				if err := openbao.Init(true); err != nil {
					fmt.Fprintf(os.Stderr, "\n❌ OpenBao initialization failed: %v\n", err)
					fmt.Println("Aborting `infra run`. Once the cause is resolved, re-run this command")
					fmt.Println("or run `./mayfly setup openbao init` manually.")
					return
				}
				fmt.Println()
			case pf.OK:
				// C2 consistent — proceed; the openbao-unseal sidecar keeps it unsealed.
				// A populated Note means token validity could not be re-confirmed
				// (transient API error): still safe to proceed, but tell the user.
				if pf.Note != "" {
					fmt.Println()
					fmt.Println(pf.Note)
				}
			default:
				fmt.Println()
				fmt.Println(pf.Advice)
				fmt.Println("\nThe other services were NOT started. Resolve the above, then run `./mayfly infra run -d` again.")
				return
			}
		}

		// Always use detached mode to avoid dependency issues
		// If user wants to see logs, we'll show them after containers are started
		convertedServiceName := convertServiceNameForDockerCompose(ServiceName)
		cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s up -d %s", ProjectName, DockerFilePath, convertedServiceName)

		// // If there are additional arguments, treat them as services or additional commands and add them to the existing command with an additional
		// if len(args) > 0 {
		// 	cmdStr += args[0]

		// 	// Explicitly passing the service name as a filter (--service) option or argument would be fine.
		// 	// serviceName := args[0]
		// 	// cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s up %s", ProjectName, DockerFilePath, serviceName)
		// }

		//fmt.Println(cmdStr)
		common.SysCall(cmdStr)

		// If user didn't explicitly request detached mode, show logs
		if !DetachMode {
			fmt.Println("\n[Showing container logs - Press Ctrl+C to stop viewing logs]")
			fmt.Println()
			logCmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s logs -f %s", ProjectName, DockerFilePath, convertedServiceName)
			common.SysCall(logCmdStr)
		} else {
			// Show log command guidance for detached mode
			fmt.Println("\n📋 Log Commands:")
			fmt.Println("  ./mayfly infra logs                    # View all service logs")
			fmt.Println("  ./mayfly infra logs -s <service-name>  # View specific service logs")
			fmt.Println("  ./mayfly infra info                    # Check system status")
			fmt.Println()
		}
	},
}

var DetachMode bool

func init() {
	dockerCmd.AddCommand(runCmd)

	// background mode
	runCmd.Flags().BoolVarP(&DetachMode, "detach", "d", false, "Detached mode: Run containers in the background without showing logs")

	// // ServiceName is used when you want to specify only a specific service
	// runCmd.Flags().StringVarP(&ServiceName, "service", "s", "", "Want to target only one specific service(Default : all)")
}
