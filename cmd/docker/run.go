package docker

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/cm-mayfly/cm-mayfly/internal/openbao"
	"github.com/spf13/cobra"
)

// showServiceInfo displays service information grouped by category
func showServiceInfo(services map[string]ServiceInfo) {
	fmt.Println("🚀 Services to be started:")
	fmt.Println()

	// Group services by category. Sorting each group by name keeps the listing
	// stable: services arrive in a map, and a map hands out its keys in a
	// different order on every run, so the lines shuffled between invocations.
	categories := make(map[string][]ServiceInfo)
	for _, service := range services {
		categories[service.Category] = append(categories[service.Category], service)
	}
	for name := range categories {
		group := categories[name]
		sort.Slice(group, func(i, j int) bool { return group[i].Name < group[j].Name })
	}

	totalServices := len(services)
	fmt.Printf("📊 Total Services: %d\n\n", totalServices)

	// Display services by category. Empty categories are skipped, so a lineup
	// without an object store prints no empty Object Storage heading.
	for _, catInfo := range categoryDisplayOrder {
		if grouped, exists := categories[catInfo.Name]; exists {
			printServiceCategory(catInfo.Icon, catInfo.Name, grouped)
			delete(categories, catInfo.Name)
		}
	}

	// Anything left carries a category the ordered list does not know about.
	// Print it rather than dropping the services from the summary: the user is
	// about to start them either way.
	remaining := make([]string, 0, len(categories))
	for name := range categories {
		remaining = append(remaining, name)
	}
	sort.Strings(remaining)
	for _, name := range remaining {
		printServiceCategory(unknownCategoryIcon, name, categories[name])
	}
}

// printServiceCategory prints one category heading and its service table, sized
// to the widest service name and image it holds.
func printServiceCategory(icon, name string, services []ServiceInfo) {
	fmt.Printf("%s %s (%d services)\n", icon, name, len(services))

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

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Install and Run Cloud-Migrator System",
	Long:  `Install and Run Cloud-Migrator System`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Install and Run Cloud-Migrator System]")
		fmt.Println()

		// Resolve -s first so an unusable value stops the command before anything
		// is started. An empty -s means "every service".
		targets, err := resolveSelectedServices()
		if err != nil {
			fmt.Printf("⚠️ %v\n", err)
			return
		}

		// Parse docker-compose.yaml to show service information
		services, err := parseDockerComposeImages()
		if err != nil {
			fmt.Printf("⚠️ Failed to parse docker-compose.yaml: %v\n", err)
			fmt.Printf("🔄 Proceeding with regular run...\n")
		} else {
			// If specific services are requested, only show those. resolveServices
			// has already confirmed every name exists.
			if len(targets) > 0 {
				selected := make(map[string]ServiceInfo, len(targets))
				for _, name := range targets {
					selected[name] = services[name]
				}
				services = selected
			}

			// Show service information
			showServiceInfo(services)
		}

		// Ask user for confirmation before proceeding with installation
		fmt.Print("\nDo you want to proceed with the installation? (y/N): ")
		var response string
		_, _ = fmt.Scanln(&response)

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
		if len(targets) == 0 {
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
		if err := runCompose(append([]string{"up", "-d"}, targets...)...); err != nil {
			fmt.Printf("\n❌ docker compose up failed: %v\n", err)
			return
		}

		// If user didn't explicitly request detached mode, show logs
		if !DetachMode {
			fmt.Println("\n[Showing container logs - Press Ctrl+C to stop viewing logs]")
			fmt.Println()
			if err := runCompose(append([]string{"logs", "-f"}, targets...)...); err != nil {
				fmt.Printf("❌ docker compose logs failed: %v\n", err)
			}
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

	// -s is a persistent flag on the parent `infra` command, so it is shared by
	// every subcommand rather than redeclared here.
}
