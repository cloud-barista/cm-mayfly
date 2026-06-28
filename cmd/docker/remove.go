package docker

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// Host data layout. Every top-level service keeps its own data (including the
// data of its dependencies such as DBs and etcd) aggregated under
// conf/docker/data/<service>/. The remove command relies on this convention so
// it can wipe host data by simply listing that directory, with OpenBao as the
// only exception that --clean-db keeps. See the FR-MM1-CLI-003-02 design.
const (
	hostDataDirName = "data"    // directory under the compose file dir
	openbaoDirName  = "openbao" // single exception preserved by --clean-db
)

// removeDocsLink points to the user guide section describing the remove
// command. It is appended to every command-definition message.
const removeDocsLink = "https://github.com/MZC-CSC/cm-mayfly/blob/develop/docs/cm-mayfly-infra.md#cloud-migrator-삭제인프라-구축-환경-정리"

// Command-definition messages. Each command prints exactly one of these
// paragraphs up front; there is no per-service dynamic messaging.
const (
	msgRemoveDefault = `[mayfly infra remove]
Stops and removes Cloud-Migrator containers. Images, volumes, and host data are preserved.
Use 'mayfly infra run' to start the system again.
Details: ` + removeDocsLink

	msgRemoveCleanDB = `[mayfly infra remove --clean-db]
Removes containers, images, named volumes, networks, and DB host data.
OpenBao credentials are preserved. Use --clean-all to remove OpenBao as well.
Details: ` + removeDocsLink

	msgRemoveCleanAll = `[mayfly infra remove --clean-all]
Removes containers, images, named volumes, networks, DB host data, and OpenBao credentials.
You must run the OpenBao initialization again when rebuilding.
Details: ` + removeDocsLink
)

var cleanDBFlag bool
var cleanAllFlag bool
var yesFlag bool
var dryRunFlag bool

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Stop and Remove Cloud-Migrator System or specific services",
	Long: `Stop and Remove Cloud-Migrator System or specific services.

By default only containers are stopped and removed; images, volumes, and host
data are preserved (equivalent to 'docker compose down').

  --clean-db   Also remove images, named volumes, and DB host data
               (conf/docker/data/* except openbao). OpenBao credentials kept.
  --clean-all  Everything --clean-db removes, plus the openbao host data.
               A full re-initialization is required afterwards.
  -s, --service <name>  Target specific services only. With --clean-db it also
               wipes conf/docker/data/<name>/. Cannot be combined with --clean-all.
  -y, --yes    Skip the confirmation prompt (for automation).
  --dry-run    Print the commands that would run, without executing them.`,
	Run: func(cmd *cobra.Command, args []string) {
		services := splitServices(ServiceName)

		// -s cannot be combined with --clean-all: the intent is ambiguous.
		if len(services) > 0 && cleanAllFlag {
			fmt.Println("\nError: the -s option cannot be combined with --clean-all.")
			fmt.Println()
			fmt.Println("  To reinitialize OpenBao only:   mayfly infra remove -s openbao --clean-db")
			fmt.Println("  For the full environment + OpenBao:   mayfly infra remove --clean-all")
			fmt.Println()
			return
		}

		// Print the single command-definition paragraph.
		switch {
		case cleanAllFlag:
			fmt.Printf("\n%s\n", msgRemoveCleanAll)
		case cleanDBFlag:
			fmt.Printf("\n%s\n", msgRemoveCleanDB)
		default:
			fmt.Printf("\n%s\n", msgRemoveDefault)
		}
		if len(services) > 0 {
			fmt.Printf("Target service(s): %s\n", strings.Join(services, " "))
		}
		fmt.Println()

		// Build the docker compose command(s) and the host data wipe targets.
		composeCmds := buildComposeCommands(services)
		hostTargets, err := hostDataTargets(services)
		if err != nil {
			fmt.Printf("Failed to inspect host data directory: %v\n", err)
			return
		}

		// --dry-run: show everything that would run and stop.
		if dryRunFlag {
			fmt.Println("[dry-run] The following commands would be executed:")
			for _, c := range composeCmds {
				fmt.Printf("  %s\n", c)
			}
			for _, t := range hostTargets {
				abs, _ := filepath.Abs(t)
				fmt.Printf("  sudo rm -rf %s\n", abs)
			}
			fmt.Println("\n[dry-run] No changes were made.")
			printDependencyHint(services)
			return
		}

		// Confirmation prompt (unless -y). Stronger wording for destructive modes.
		if !yesFlag {
			var prompt string
			switch {
			case cleanAllFlag:
				prompt = "This will remove OpenBao credentials, DB data, and all host bind mounts. You must redo tumblebug-init from scratch. Proceed? (y/N): "
			case cleanDBFlag:
				prompt = "This will remove DB data, images, named volumes, and networks (OpenBao credentials are preserved). Proceed? (y/N): "
			default:
				prompt = "Do you want to proceed with the removal? (y/N): "
			}
			if !confirm(prompt) {
				fmt.Println("Removal cancelled.")
				return
			}
		}

		// Execute compose command(s) first, then wipe host data.
		for _, c := range composeCmds {
			common.SysCall(c)
		}
		for _, t := range hostTargets {
			abs, _ := filepath.Abs(t)
			common.SysCall(fmt.Sprintf("sudo rm -rf %s", abs))
		}

		// --clean-all also removed the OpenBao host data + volume, so the
		// VAULT_TOKEN in .env now points at credentials that no longer exist.
		// Clear just that key (every other user setting is preserved) so the
		// next `setup openbao init` runs cleanly without needing --force.
		if cleanAllFlag {
			envFile := filepath.Join("conf", "docker", ".env")
			if err := clearEnvKey(envFile, "VAULT_TOKEN"); err != nil {
				fmt.Printf("warn: failed to clear VAULT_TOKEN from %s: %v\n", envFile, err)
			} else {
				fmt.Println("Cleared VAULT_TOKEN from .env (OpenBao credentials were removed).")
			}
		}

		printDependencyHint(services)

		SysCallDockerComposePsWithAll(false)
	},
}

// clearEnvKey rewrites the .env at path, setting key to an empty value while
// preserving every other line. If the key is absent the file is left unchanged.
func clearEnvKey(path, key string) error {
	// path is a fixed internal .env location (conf/docker/.env), not user input.
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
			lines[i] = key + "="
			found = true
		}
	}
	if !found {
		return nil
	}
	// .env holds secrets (VAULT_TOKEN); keep it owner-only. The file already
	// exists here, so this preserves its current mode rather than widening it.
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
}

// printDependencyHint reminds the user that, with the flat data layout, a
// service-scoped --clean-db removes only that service's own conf/docker/data/<svc>;
// the data of any services it depends on lives under their own flat folders and
// is not touched. This is the first-phase guidance; an automatic --cascade that
// walks the depends_on graph is a planned follow-up.
func printDependencyHint(services []string) {
	if len(services) == 0 || !(cleanDBFlag || cleanAllFlag) {
		return
	}
	fmt.Println()
	fmt.Println("Note: with the flat data layout, --clean-db removes only the targeted")
	fmt.Println("service's own data under conf/docker/data/<service>. A service it depends")
	fmt.Println("on keeps its data in its own folder; remove those explicitly if needed, e.g.")
	fmt.Println("  mayfly infra remove -s \"<service> <dependency-service>\" --clean-db")
}

// splitServices normalizes the -s value (comma- or space-separated) into a slice.
func splitServices(serviceName string) []string {
	if strings.TrimSpace(serviceName) == "" {
		return nil
	}
	fields := strings.FieldsFunc(serviceName, func(r rune) bool {
		return r == ',' || r == ' '
	})
	var out []string
	for _, f := range fields {
		if t := strings.TrimSpace(f); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// confirm reads a yes/no answer from stdin.
func confirm(prompt string) bool {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// buildComposeCommands returns the docker compose command(s) to run for the
// requested scope. Whole-system removal uses `down`; a service-scoped removal
// stops and removes only the named services.
func buildComposeCommands(services []string) []string {
	wipeImagesVolumes := cleanDBFlag || cleanAllFlag

	if len(services) == 0 {
		opts := "--remove-orphans"
		if wipeImagesVolumes {
			opts = "--volumes --rmi all --remove-orphans"
		}
		return []string{fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s down %s", ProjectName, DockerFilePath, opts)}
	}

	svc := convertServiceNameForDockerCompose(ServiceName)
	stop := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s stop %s", ProjectName, DockerFilePath, svc)
	rmOpts := "-s -f"
	if cleanDBFlag {
		rmOpts = "-s -v -f" // also drop anonymous/named volumes attached to the service
	}
	rm := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s rm %s %s", ProjectName, DockerFilePath, rmOpts, svc)
	return []string{stop, rm}
}

// hostDataTargets returns the host directories to wipe for the requested scope.
//
//   - default (no --clean-db/--clean-all): nothing is wiped.
//   - --clean-db whole system: every conf/docker/data/* directory except openbao.
//   - --clean-all whole system: every conf/docker/data/* directory (openbao included).
//   - -s <svc> with --clean-db: only conf/docker/data/<svc>/ for each service.
//
// Listing the data directory means new services are handled automatically once
// their data lands under conf/docker/data/<service>/ (host data aggregation).
func hostDataTargets(services []string) ([]string, error) {
	if !cleanDBFlag && !cleanAllFlag {
		return nil, nil
	}

	dataRoot := filepath.Join(filepath.Dir(DockerFilePath), hostDataDirName)

	if len(services) > 0 {
		// Service-scoped wipe only applies under --clean-db (validated above to
		// be incompatible with --clean-all).
		var targets []string
		for _, s := range services {
			targets = append(targets, filepath.Join(dataRoot, s))
		}
		return targets, nil
	}

	entries, err := os.ReadDir(dataRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var targets []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !cleanAllFlag && e.Name() == openbaoDirName {
			continue // --clean-db preserves OpenBao credentials
		}
		targets = append(targets, filepath.Join(dataRoot, e.Name()))
	}
	return targets, nil
}

func init() {
	dockerCmd.AddCommand(removeCmd)

	pf := removeCmd.PersistentFlags()
	pf.BoolVar(&cleanDBFlag, "clean-db", false, "Remove images, named volumes, and DB host data (conf/docker/data/* except openbao). OpenBao credentials preserved")
	pf.BoolVar(&cleanAllFlag, "clean-all", false, "Everything --clean-db removes, plus openbao host data (full reset)")
	pf.BoolVarP(&yesFlag, "yes", "y", false, "Skip the confirmation prompt")
	pf.BoolVar(&dryRunFlag, "dry-run", false, "Print the commands that would run, without executing them")
}
