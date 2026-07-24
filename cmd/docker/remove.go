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
// paragraphs up front; there is no per-service dynamic messaging, so the
// paragraphs use a <service> placeholder and the concrete names are printed on
// the "Target service(s):" line that follows.
//
// Every paragraph states what is removed and what is kept as two labelled
// lists rather than as prose. The prose form said "Stops and removes … Images,
// volumes, and host data are preserved", which is accurate but reads as two
// sentences cancelling each other out: the command is called `remove`, the
// first sentence confirms that reading, and the second takes it back — leaving
// the user unable to tell what the unit of removal actually is. The lists say
// it once, in one place, per path.
//
// Volumes are deliberately not mentioned. The compose file declares no named or
// anonymous volumes (every service keeps its data in a bind mount under
// conf/docker/data/<service>), so naming them would announce the removal of
// something that does not exist.
const (
	msgRemoveDefault = `[mayfly infra remove]
Stops and removes the Cloud-Migrator containers. Data is NOT deleted.

  Removed   containers, project network, orphan containers
  Kept      images and all host data under conf/docker/data/
            (DB contents, OpenBao credentials, service state)

'mayfly infra run' restarts the system with the existing data.

To delete data as well:
  --clean-db    + images and host data (OpenBao credentials kept)
  --clean-all   + OpenBao credentials (full reset)

Details: ` + removeDocsLink

	// A service-scoped removal used to print the paragraph above verbatim. Its
	// opening line names "the Cloud-Migrator containers", which reads as all of
	// them; the "Target service(s):" line that follows narrows the scope, but a
	// reader who has already taken the first sentence at face value sees that
	// line as extra detail rather than as the correction it is. So the scope
	// goes in the opening sentence instead.
	msgRemoveDefaultService = `[mayfly infra remove -s <service>]
Stops and removes only the targeted container(s).
Other services keep running. Data is NOT deleted.

  Removed   the targeted container(s) only
  Kept      project network (other services stay connected), images,
            and host data under conf/docker/data/<service>

'mayfly infra run -d -s <service>' brings it back with the existing data.

To also delete the image and data:
  --clean-db    + the service's image and conf/docker/data/<service>

Details: ` + removeDocsLink

	msgRemoveCleanDB = `[mayfly infra remove --clean-db]
Removes every service and all host data except the OpenBao credentials.
This cannot be undone.

  Removed   containers, project network, images,
            and every conf/docker/data/* directory (DB contents, service state)
  Kept      conf/docker/data/openbao/ — OpenBao credentials

Use --clean-all to remove the OpenBao credentials as well.

Details: ` + removeDocsLink

	// A service-scoped --clean-db removes the same kinds of thing as the
	// whole-system one, only narrowed to the named services — including their
	// images, which it used to leave behind. Leaving them behind is what made
	// "I removed it and pulled again but the old build is still running" a
	// recurring question: a tag that moves (edge, latest, a re-pushed one) is
	// not re-pulled while a local image with that tag exists.
	msgRemoveCleanDBService = `[mayfly infra remove -s <service> --clean-db]
Removes the targeted service(s), their images, and their host data.
This cannot be undone.

  Removed   the targeted container(s), their images,
            and conf/docker/data/<service>
  Kept      project network and every other service's data

The image is removed so the next 'mayfly infra run' pulls it again. To restart
without re-downloading it, run the same command without --clean-db.

Details: ` + removeDocsLink

	msgRemoveCleanAll = `[mayfly infra remove --clean-all]
Full reset — removes every service, all host data, and the OpenBao credentials.
This cannot be undone.

  Removed   containers, project network, images,
            every conf/docker/data/* directory including openbao,
            and VAULT_TOKEN in conf/docker/.env
  Kept      nothing

'mayfly infra run' re-initializes OpenBao automatically, but the cb-tumblebug
credentials and assets are gone — run 'mayfly setup tumblebug-init' again.

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

By default only containers and the project network are removed; images and the
host data under conf/docker/data/ are kept (equivalent to 'docker compose down').

  --clean-db   Also remove the images and the host data under
               conf/docker/data/* (except openbao). OpenBao credentials kept.
  --clean-all  Everything --clean-db removes, plus the openbao host data and the
               VAULT_TOKEN entry in .env. A full re-initialization follows.
  -s, --service <name>  Target specific services only; the others keep running
               and the project network is preserved. With --clean-db it also
               removes those services' images and conf/docker/data/<name>/.
               Cannot be combined with --clean-all.
  -y, --yes    Skip the confirmation prompt (for automation).
  --dry-run    Print the commands that would run, without executing them.`,
	Run: func(cmd *cobra.Command, args []string) {
		// resolveServices is what keeps the scope honest: an empty -s means "the
		// whole environment", and anything else must name services that actually
		// exist. A value that splits to nothing (-s "," or -s " ") is rejected
		// rather than silently widened to everything.
		services, err := resolveSelectedServices()
		if err != nil {
			fmt.Printf("\n❌ %v\n", err)
			return
		}

		// -s cannot be combined with --clean-all: the intent is ambiguous.
		if len(services) > 0 && cleanAllFlag {
			fmt.Println("\nError: the -s option cannot be combined with --clean-all.")
			fmt.Println()
			fmt.Println("  To reinitialize OpenBao only:   mayfly infra remove -s openbao --clean-db")
			fmt.Println("  For the full environment + OpenBao:   mayfly infra remove --clean-all")
			fmt.Println()
			return
		}

		// Print the single command-definition paragraph. Both service-scoped
		// paths get their own paragraph: what they remove, and what they leave
		// running, differs from the whole-system paths in ways the shared text
		// cannot state without being wrong for one of them.
		switch {
		case cleanAllFlag:
			fmt.Printf("\n%s\n", msgRemoveCleanAll)
		case cleanDBFlag && len(services) > 0:
			fmt.Printf("\n%s\n", msgRemoveCleanDBService)
		case cleanDBFlag:
			fmt.Printf("\n%s\n", msgRemoveCleanDB)
		case len(services) > 0:
			fmt.Printf("\n%s\n", msgRemoveDefaultService)
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
		imageTargets, err := imageRemovalTargets(services)
		if err != nil {
			fmt.Printf("Failed to resolve the images of the targeted service(s): %v\n", err)
			return
		}

		// --dry-run: show everything that would run and stop.
		if dryRunFlag {
			fmt.Print(dryRunPlan(composeCmds, imageTargets, hostTargets))
			printDependencyHint(services)
			return
		}

		// Confirmation prompt (unless -y). Stronger wording for destructive modes.
		// The prompt is the last thing read before the command acts, so it
		// repeats the outcome rather than asking an unqualified "proceed?". The
		// default path needs this most: it is the one where nothing is lost, and
		// the bare question used to give a user about to press y no way of
		// knowing that.
		if !yesFlag {
			var prompt string
			switch {
			case cleanAllFlag:
				prompt = "Delete everything including the OpenBao credentials? You must run tumblebug-init again. (y/N): "
			case cleanDBFlag && len(services) > 0:
				prompt = "Delete the targeted service(s), their images, and their data under conf/docker/data/? (y/N): "
			case cleanDBFlag:
				prompt = "Delete the containers, images, and host data (OpenBao credentials kept)? (y/N): "
			case len(services) > 0:
				prompt = "Stop and remove the targeted container(s)? Other services and all data will be kept. (y/N): "
			default:
				prompt = "Stop and remove the containers? Data will be kept. (y/N): "
			}
			if !confirm(prompt) {
				fmt.Println("Removal cancelled.")
				return
			}
		}

		// A failing step means the environment is only partly torn down, so the
		// sequence stops instead of carrying on and reporting success.
		if len(hostTargets) > 0 {
			if err := ensureSudoAvailable(); err != nil {
				fmt.Printf("\n❌ %v\n", err)
				fmt.Println("\nNothing was removed.")
				return
			}
		}

		// Execute compose command(s) first, then wipe host data.
		for _, c := range composeCmds {
			if err := runCompose(c...); err != nil {
				fmt.Printf("\n❌ %s failed: %v\n", displayCommand("docker", composeArgs(c...)), err)
				fmt.Println("Stopping here — the host data was NOT removed.")
				return
			}
		}
		// Images, once their containers are gone. Unlike the steps around it a
		// failure here is reported and stepped over rather than fatal: an image
		// that another container still uses cannot be removed, and that is a
		// reason to leave it alone, not to abandon a teardown that has already
		// removed the containers. `rmi` is deliberately not forced for the same
		// reason — forcing it would untag an image out from under whatever is
		// still running on it.
		for _, img := range imageTargets {
			if err := common.RunCommand("docker", []string{"rmi", img}, nil); err != nil {
				fmt.Printf("warn: could not remove the image %s: %v\n", img, err)
				fmt.Println("      (it is probably still in use by another container)")
			}
		}

		for _, t := range hostTargets {
			abs, _ := filepath.Abs(t)
			if err := common.RunCommand("sudo", []string{"rm", "-rf", abs}, nil); err != nil {
				fmt.Printf("\n❌ failed to remove %s: %v\n", abs, err)
				fmt.Println("Stopping here — the removal is incomplete.")
				return
			}
		}

		// --clean-all also removed the OpenBao host data + volume, so the
		// VAULT_TOKEN in .env now points at credentials that no longer exist.
		// Clear just that key (every other user setting is preserved) so the
		// next `setup openbao init` runs cleanly without needing --force.
		if cleanAllFlag {
			envFile := envFilePath()
			switch found, err := clearEnvKey(envFile, "VAULT_TOKEN"); {
			case err != nil:
				fmt.Printf("warn: failed to clear VAULT_TOKEN from %s: %v\n", envFile, err)
			case found:
				fmt.Println("Cleared VAULT_TOKEN from .env (OpenBao credentials were removed).")
			default:
				// Nothing was cleared, so saying otherwise would be a plain
				// falsehood — and a misleading one, because a user who reads it
				// concludes .env is consistent with the wipe that just happened.
				fmt.Printf("No VAULT_TOKEN entry found in %s — nothing to clear.\n", envFile)
			}
		}

		printDependencyHint(services)

		SysCallDockerComposePsWithAll(false)
	},
}

// dryRunPlan renders everything the command would do, in the order it would do
// it. Building it as a string rather than printing inline keeps it checkable:
// the point of --dry-run is that this list and the real run agree, and a list
// nothing can compare against is exactly how the .env edit went unannounced.
func dryRunPlan(composeCmds [][]string, imageTargets []string, hostTargets []string) string {
	var b strings.Builder
	b.WriteString("[dry-run] The following commands would be executed:\n")
	for _, c := range composeCmds {
		fmt.Fprintf(&b, "  %s\n", displayCommand("docker", composeArgs(c...)))
	}
	for _, img := range imageTargets {
		fmt.Fprintf(&b, "  docker rmi %s\n", img)
	}
	for _, t := range hostTargets {
		abs, _ := filepath.Abs(t)
		fmt.Fprintf(&b, "  sudo rm -rf %s\n", abs)
	}
	// --clean-all edits .env as its last step. A dry-run that lists the removals
	// but not that edit understates what the real run does, and .env is the one
	// thing here a user may have customised by hand.
	if cleanAllFlag {
		abs, _ := filepath.Abs(envFilePath())
		fmt.Fprintf(&b, "  clear VAULT_TOKEN in %s\n", abs)
	}
	b.WriteString("\n[dry-run] No changes were made.\n")
	return b.String()
}

// envFilePath is the .env the remove command may edit — the same file every
// other infra subcommand reads.
func envFilePath() string {
	return filepath.Join("conf", "docker", ".env")
}

// clearEnvKey rewrites the .env at path, setting key to an empty value while
// preserving every other line. If the key is absent the file is left unchanged.
//
// found reports whether the key was actually there. It is separate from err
// because "the file was fine and the key was not in it" is a success as far as
// the file is concerned, but it is not the same outcome as clearing the key —
// and the caller prints a different sentence for each. Folding the two into a
// single nil error is what made the command announce "Cleared VAULT_TOKEN from
// .env" for an .env that never had the line.
func clearEnvKey(path, key string) (found bool, err error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is the internal .env next to the compose file, not user input
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
			lines[i] = key + "="
			found = true
		}
	}
	if !found {
		return false, nil
	}
	// .env holds secrets (VAULT_TOKEN); writeEnvFile replaces it atomically and
	// keeps it owner-only.
	if err := writeEnvFile(path, []byte(strings.Join(lines, "\n"))); err != nil {
		return false, err
	}
	return true, nil
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
// Each entry is an argument vector for `docker compose -f <file> …`, executed
// directly rather than through a shell.
func buildComposeCommands(services []string) [][]string {
	wipeImagesVolumes := cleanDBFlag || cleanAllFlag

	if len(services) == 0 {
		down := []string{"down"}
		if wipeImagesVolumes {
			down = append(down, "--volumes", "--rmi", "all")
		}
		down = append(down, "--remove-orphans")
		return [][]string{down}
	}

	stop := append([]string{"stop"}, services...)

	rm := []string{"rm", "-s", "-f"}
	if cleanDBFlag {
		// also drop anonymous/named volumes attached to the service
		rm = []string{"rm", "-s", "-v", "-f"}
	}
	rm = append(rm, services...)

	return [][]string{stop, rm}
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
			target := filepath.Join(dataRoot, s)
			if err := assertUnderDataRoot(dataRoot, target); err != nil {
				return nil, err
			}
			targets = append(targets, target)
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
		target := filepath.Join(dataRoot, e.Name())
		if err := assertUnderDataRoot(dataRoot, target); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

// imageRemovalTargets returns the images to remove with `docker rmi`.
//
// Only the service-scoped --clean-db path needs them. The whole-system paths
// pass --rmi all to `docker compose down`, which removes the images as part of
// the same call, and the paths without --clean-db remove no images at all.
//
// Why this exists: `docker compose rm` has no --rmi, and --rmi belongs to
// `down`, which cannot be narrowed to a service. So a service-scoped removal
// that also drops the image has to name the image itself — and -s gives a
// service name, which the image cannot be derived from (cm-butterfly-db runs
// postgres:14-alpine, cb-tumblebug-etcd runs gcr.io/etcd-development/etcd).
// The mapping exists only in the compose file, so it is read from there.
func imageRemovalTargets(services []string) ([]string, error) {
	if !cleanDBFlag || len(services) == 0 {
		return nil, nil
	}

	// The same parse resolveServices validated the names against, so the
	// services whose images are removed are exactly the ones that were checked.
	available, err := parseDockerComposeImages()
	if err != nil {
		return nil, err
	}

	var images []string
	seen := make(map[string]bool, len(services))
	for _, s := range services {
		info, ok := available[s]
		if !ok || info.Image == "" {
			// resolveServices already rejected unknown names, so this is a
			// service declared with `build:` and no image. Nothing to remove.
			continue
		}
		// An unresolved ${VAR} would be handed to `docker rmi` as a literal and
		// fail, and the failure is non-fatal — so the image would quietly stay
		// behind, which is the exact outcome this change exists to prevent. The
		// compose file is read as YAML and interpolation is not applied to it,
		// so say plainly that this one cannot be resolved.
		if strings.Contains(info.Image, "${") {
			return nil, fmt.Errorf("the image of service %q is declared as %q and still contains an unresolved variable.\n"+
				"Remove it manually with `docker rmi`, or pin the tag in the compose file", s, info.Image)
		}
		if seen[info.Image] {
			continue // two services sharing an image: remove it once
		}
		seen[info.Image] = true
		images = append(images, info.Image)
	}
	return images, nil
}

// assertUnderDataRoot refuses any wipe target that is not inside the host data
// directory.
//
// resolveServices already guarantees a -s value names a service declared in the
// compose file, so a traversal sequence such as -s "../../.." cannot reach this
// point. This is the second line of defence: the cost of being wrong here is a
// `sudo rm -rf` outside the data directory, so the path is checked on its own
// terms rather than trusted because an earlier stage checked something else.
func assertUnderDataRoot(dataRoot, target string) error {
	absRoot, err := filepath.Abs(dataRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve the data directory %s: %w", dataRoot, err)
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("failed to resolve the removal target %s: %w", target, err)
	}

	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return fmt.Errorf("failed to compare %s against the data directory %s: %w", absTarget, absRoot, err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refusing to remove %s: it lies outside the data directory %s", absTarget, absRoot)
	}
	return nil
}

// ensureSudoAvailable checks that sudo can run without prompting for a password
// before the removal sequence starts.
//
// The wipe step shells out to `sudo rm -rf`. On an account without NOPASSWD that
// call fails, and because nothing used to inspect the exit status the command
// still printed its success message — leaving the operator believing a clean
// rebuild happened while the data was still on disk. Checking up front means the
// command either does what it says or says why it cannot.
func ensureSudoAvailable() error {
	if err := common.RunCommand("sudo", []string{"-n", "true"}, nil); err != nil {
		return fmt.Errorf("sudo is required to remove the host data under %s, but it cannot run without a password.\n\n"+
			"Either allow it without a prompt:\n"+
			"  add a NOPASSWD entry for your account with `sudo visudo`\n"+
			"or run this command as a user that already has it:\n"+
			"  sudo ./mayfly infra remove …\n\n"+
			"underlying error: %w", hostDataDirName, err)
	}
	return nil
}

func init() {
	dockerCmd.AddCommand(removeCmd)

	pf := removeCmd.PersistentFlags()
	pf.BoolVar(&cleanDBFlag, "clean-db", false, "Also remove the images and the host data under conf/docker/data/* (except openbao). With -s, scoped to the named services. OpenBao credentials preserved")
	pf.BoolVar(&cleanAllFlag, "clean-all", false, "Everything --clean-db removes, plus openbao host data and the .env VAULT_TOKEN (full reset)")
	pf.BoolVarP(&yesFlag, "yes", "y", false, "Skip the confirmation prompt")
	pf.BoolVar(&dryRunFlag, "dry-run", false, "Print the commands that would run, without executing them")

	// --clean-db and --clean-all describe two different amounts of destruction,
	// and passing both used to run --clean-all without a word. The difference
	// between them is precisely whether the OpenBao credentials survive, so a
	// user who wrote --clean-db meant to keep them and lost them anyway. Cobra
	// rejects the combination up front rather than picking one.
	removeCmd.MarkFlagsMutuallyExclusive("clean-db", "clean-all")
}
