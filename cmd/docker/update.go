package docker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// DockerHubTagInfo represents Docker Hub tag information
type DockerHubTagInfo struct {
	Name        string `json:"name"`
	LastUpdated string `json:"last_updated"`
}

// DockerHubTagResponse represents Docker Hub API response for tag list
type DockerHubTagResponse struct {
	Results []DockerHubTagInfo `json:"results"`
}

// DockerHubTagDetail represents detailed Docker Hub tag information
type DockerHubTagDetail struct {
	Name        string `json:"name"`
	LastUpdated string `json:"last_updated"`
}

// DockerHubTagDetailResponse represents Docker Hub API response for specific tag
type DockerHubTagDetailResponse struct {
	LastUpdated string `json:"last_updated"`
	// Digest identifies the content the tag currently points at. A moving tag
	// such as edge or latest keeps its name while this changes, so it is the
	// only way to tell whether such a tag has actually been rebuilt.
	Digest string `json:"digest"`
}

// dockerHubTag is what one tag lookup tells us. Both fields are best-effort:
// an empty value means the lookup did not answer for it, never that the remote
// image is missing.
type dockerHubTag struct {
	LastUpdated string
	Digest      string
}

// dockerHubRepositoryPath maps an image name to its path in the Docker Hub API.
//
// Official images are addressed as "library/<name>" there, even though they are
// pulled as a bare name. Without the prefix the API answers 404 for every one
// of them, which is why postgres, redis, mysql and etcd never showed a value in
// the Latest column.
func dockerHubRepositoryPath(imageName string) string {
	if strings.Contains(imageName, "/") {
		return imageName
	}
	return "library/" + imageName
}

// fetchDockerHubTag reads when a tag was last pushed and what content it points
// at. One request answers both; the digest is what makes a moving tag such as
// edge or latest comparable at all.
func fetchDockerHubTag(imageName, tag string) (dockerHubTag, error) {
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags/%s/",
		dockerHubRepositoryPath(imageName), tag)

	// The shared client carries a request timeout. The bare http.Get this
	// replaced used the default client, which has none, so a slow or
	// unresponsive Docker Hub left `infra update` waiting with no way out.
	resp, err := common.NewHTTPClient().R().Get(url)
	if err != nil {
		return dockerHubTag{}, fmt.Errorf("failed to fetch Docker Hub API: %v", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return dockerHubTag{}, fmt.Errorf("Docker Hub API returned status %d", resp.StatusCode())
	}

	var tagResponse DockerHubTagDetailResponse
	if err := json.Unmarshal(resp.Body(), &tagResponse); err != nil {
		return dockerHubTag{}, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return dockerHubTag{
		LastUpdated: tagResponse.LastUpdated,
		Digest:      tagResponse.Digest,
	}, nil
}

// fetchTagInfo is the Docker Hub lookup used by the version check, indirected
// so tests can exercise the comparison without reaching the network.
var fetchTagInfo = fetchDockerHubTag

// localImageDigest reports the registry digest of the image held locally under
// imageName:tag, or "" when there is nothing to compare against.
//
// An image built on this host, or one pulled before digests were recorded, has
// no RepoDigests entry. That is not an error and must not be read as "the local
// copy differs" — the caller falls back to comparing tag names instead.
func localImageDigest(imageName, tag string) string {
	out, err := common.RunCommandOutput("docker",
		[]string{"image", "inspect", fmt.Sprintf("%s:%s", imageName, tag),
			"--format", "{{if .RepoDigests}}{{index .RepoDigests 0}}{{end}}"}, nil)
	if err != nil {
		return ""
	}

	ref := strings.TrimSpace(string(out))
	if _, digest, found := strings.Cut(ref, "@"); found {
		return digest
	}
	return ""
}

// shortDate trims a Docker Hub timestamp to the day. The full RFC 3339 value is
// too wide for the table and the time of day never decides anything here.
func shortDate(timestamp string) string {
	if date, _, found := strings.Cut(timestamp, "T"); found {
		return date
	}
	return timestamp
}

// getCurrentLocalVersion gets the current local image version of a service
// It first tries to get the actual running container's image, then falls back to checking local images
func getCurrentLocalVersion(imageName, tag string, serviceName string) (string, error) {
	// First, try to get the actual running container's image using docker compose ps
	// This is more accurate than docker ps --filter because it uses the service name from docker-compose.yaml
	output, err := composeOutput("ps", "--format", "json")
	if err == nil {
		// Parse JSON output to find the container for this service
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}

			var container struct {
				Name    string `json:"Name"`
				Service string `json:"Service"`
				State   string `json:"State"`
				Image   string `json:"Image"`
			}

			if err := json.Unmarshal([]byte(line), &container); err != nil {
				continue
			}

			// Check if this container belongs to the service we're looking for
			if container.Service == serviceName && container.State == "running" {
				// Extract tag from image:tag format
				parts := strings.Split(container.Image, ":")
				if len(parts) > 1 {
					return parts[len(parts)-1], nil
				}
				// If no tag, return the full image (might be image ID)
				return container.Image, nil
			}
		}
	}

	// Fallback: Check if the specific image exists locally
	output2, err := common.RunCommandOutput("docker",
		[]string{"images", "--format", "{{.Tag}}", fmt.Sprintf("%s:%s", imageName, tag)}, nil)
	if err != nil {
		// A failing `docker images` is not the same as an absent image: the
		// command exits 0 with empty output for that (handled just below).
		// Reporting "not_installed" here turned an unreachable daemon into a
		// table claiming every image was missing, which then read as "there is
		// an update for everything". Hand the failure back instead.
		return "", fmt.Errorf("could not list local images for %s:%s: %w", imageName, tag, err)
	}

	localTag := strings.TrimSpace(string(output2))
	if localTag == "" {
		// `docker images` prints nothing (and still exits 0) when the image is
		// absent, so an empty result means it is not installed.
		return "not_installed", nil
	}

	// Return the tag if image exists locally
	return localTag, nil
}

// serviceVersion is one row of the comparison table.
type serviceVersion struct {
	Service string
	Local   string // tag the running container uses, or not_installed / unknown
	Compose string // tag docker-compose.yaml asks for
	Remote  string // day Docker Hub last pushed that tag, or "-"
	Status  string
	Stale   bool   // needs a pull and a recreate
	Why     string // reason, printed under the table for the stale rows
}

// checkVersionUpdates compares every service against docker-compose.yaml and
// Docker Hub, prints the table, and returns the services that need updating.
//
// It returns the names rather than a yes/no because the caller uses them as the
// scope of the pull and the restart. That is the whole point: the earlier
// version returned a single bool, so a run without -s pulled and recreated
// *every* service, including ones the table had just shown as unchanged. What
// the user agreed to on screen and what the command then did were different
// things.
//
// Two judgements are deliberately conservative, because being wrong here
// restarts a service nobody asked to touch:
//
//   - A tag that matches by name is compared by content as well. A moving tag
//     (edge, latest) keeps its name across rebuilds, so the name alone can never
//     mark it stale. If either digest is unavailable the row stays up to date.
//   - A service whose local version could not be read is left out entirely. It
//     used to count as stale, which turned a docker hiccup into a restart.
//
// Individual Docker Hub lookups stay best-effort — an unreachable registry
// leaves "-" in the Hub column and falls back to the tag comparison. An error is
// returned only when the local lookup failed for every service examined: at that
// point nothing in the table is grounded in the real state of the host, so the
// caller must not put a confirmation prompt in front of the user based on it.
func checkVersionUpdates(services map[string]ServiceInfo) ([]string, error) {
	fmt.Println("🔍 Checking version updates...")
	fmt.Println()

	rows := make([]serviceVersion, 0, len(services))
	examined := 0
	localFailures := 0
	var firstLocalErr error

	for serviceName, serviceInfo := range services {
		// Extract image name and tag. splitImageRef handles the cases a plain
		// Split(":") gets wrong, such as a registry host that carries a port.
		imageName, composeTag := splitImageRef(serviceInfo.Image)
		if imageName == "" || composeTag == "" {
			continue
		}

		examined++

		// Get current local version (pass serviceName to get actual running container's image)
		currentVersion, err := getCurrentLocalVersion(imageName, composeTag, serviceName)
		if err != nil {
			currentVersion = "unknown"
			localFailures++
			if firstLocalErr == nil {
				firstLocalErr = fmt.Errorf("%s: %w", serviceName, err)
			}
		}

		remote, hubErr := fetchTagInfo(imageName, composeTag)

		row := serviceVersion{
			Service: serviceName,
			Local:   currentVersion,
			Compose: composeTag,
			Remote:  "-",
		}
		if hubErr == nil && remote.LastUpdated != "" {
			row.Remote = shortDate(remote.LastUpdated)
		}

		switch {
		case currentVersion == "not_installed":
			row.Stale, row.Status, row.Why = true, "✗", "not installed locally"
		case currentVersion == "unknown":
			// Left out of the update set on purpose — see the doc comment.
			row.Status, row.Why = "?", "local version could not be read"
		case currentVersion != composeTag:
			row.Stale, row.Status, row.Why = true, "●", "tag differs from docker-compose.yaml"
		default:
			localDigest := localImageDigest(imageName, composeTag)
			if hubErr == nil && remote.Digest != "" && localDigest != "" && localDigest != remote.Digest {
				row.Stale, row.Status, row.Why = true, "◆", "same tag, but Docker Hub holds different content"
			} else {
				row.Status = "✓"
			}
		}

		rows = append(rows, row)
	}

	// Every local lookup failed, so "Local" is "unknown" on every row and any
	// conclusion drawn from it is an artefact of that, not a finding. Report it
	// instead of letting the caller act on it.
	if examined > 0 && localFailures == examined {
		return nil, fmt.Errorf("could not read the locally installed image version for any of the %d service(s); is the docker daemon reachable? (first failure — %w)", examined, firstLocalErr)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Service < rows[j].Service })
	printVersionComparison(rows)

	stale := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Stale {
			stale = append(stale, row.Service)
		}
	}
	return stale, nil
}

// printVersionComparison renders the comparison table and the reasons behind it.
func printVersionComparison(rows []serviceVersion) {
	nameWidth := len("Service")
	versionWidth := len("Compose")
	remoteWidth := len("Hub updated")

	for _, row := range rows {
		if len(row.Service) > nameWidth {
			nameWidth = len(row.Service)
		}
		for _, value := range []string{row.Local, row.Compose} {
			if len(value) > versionWidth {
				versionWidth = len(value)
			}
		}
		if len(row.Remote) > remoteWidth {
			remoteWidth = len(row.Remote)
		}
	}

	rule := func(left, mid, right string) {
		fmt.Printf("%s%s%s%s%s%s%s%s%s\n", left,
			strings.Repeat("─", nameWidth+2), mid,
			strings.Repeat("─", versionWidth+2), mid,
			strings.Repeat("─", versionWidth+2), mid,
			strings.Repeat("─", remoteWidth+2), right)
	}

	fmt.Println("📊 Version Comparison:")
	rule("┌", "┬", "┐")
	fmt.Printf("│ %-*s │ %-*s │ %-*s │ %-*s │\n",
		nameWidth, "Service", versionWidth, "Local", versionWidth, "Compose", remoteWidth, "Hub updated")
	rule("├", "┼", "┤")
	for _, row := range rows {
		fmt.Printf("│ %-*s │ %-*s │ %-*s │ %-*s │ %s\n",
			nameWidth, row.Service, versionWidth, row.Local,
			versionWidth, row.Compose, remoteWidth, row.Remote, row.Status)
	}
	rule("└", "┴", "┘")
	fmt.Println()

	fmt.Println("Legend:")
	fmt.Println("✓ Up to date")
	fmt.Println("● Tag differs from docker-compose.yaml (update needed)")
	fmt.Println("◆ Same tag, but Docker Hub holds different content (update needed)")
	fmt.Println("✗ Image not installed locally")
	fmt.Println("? Local version could not be read — left out of the update")
	fmt.Println()
	fmt.Println("Hub updated is when Docker Hub last pushed the tag docker-compose.yaml asks for.")
	fmt.Println("It is information, not the verdict: the verdict comes from the columns to its left.")
	fmt.Println()

	var notes []string
	for _, row := range rows {
		if row.Why != "" {
			notes = append(notes, fmt.Sprintf("  %s %s — %s", row.Status, row.Service, row.Why))
		}
	}
	if len(notes) > 0 {
		fmt.Println("Details:")
		for _, note := range notes {
			fmt.Println(note)
		}
		fmt.Println()
	}
}

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the latest Docker images of the subsystems that make up the Cloud-Migrator system.",
	Long:  `Update the latest Docker images of the subsystems that make up the Cloud-Migrator system.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\n[Update the Docker images of the subsystems that make up the Cloud-Migrator system.]")

		// Resolve -s first so an unusable value stops the command before anything
		// is pulled or restarted. An empty -s means "every service".
		targets, err := resolveSelectedServices()
		if err != nil {
			fmt.Printf("⚠️ %v\n", err)
			return
		}

		// What the pull and the restart will actually act on. It starts as the
		// -s selection and, when -s was not given, narrows to the services the
		// version check finds stale. Empty means every service, which is only
		// reached when there is nothing to narrow by — a failed check, or the
		// user choosing to pull and restart anyway.
		scope := targets

		// Parse docker-compose.yaml to get actual image information
		services, err := parseDockerComposeImages()
		if err != nil {
			fmt.Printf("❌ Failed to parse docker-compose.yaml: %v\n", err)
			fmt.Printf("🔄 Falling back to regular pull...\n")

			// Fallback to regular pull without version check
			if err := runCompose(append([]string{"pull", "--ignore-pull-failures"}, scope...)...); err != nil {
				fmt.Printf("❌ docker compose pull failed: %v\n", err)
				return
			}
		} else {
			// If specific services are requested, only check those. resolveServices
			// has already confirmed every name exists.
			if len(targets) > 0 {
				selected := make(map[string]ServiceInfo, len(targets))
				for _, name := range targets {
					selected[name] = services[name]
				}
				services = selected
			}

			// Check for version updates
			stale, err := checkVersionUpdates(services)
			if err != nil {
				fmt.Printf("⚠️ Failed to check version updates: %v\n", err)
				fmt.Printf("🔄 Proceeding with regular pull...\n")
			} else if len(stale) == 0 {
				fmt.Println("✅ All services are up to date!")
				fmt.Print("Do you want to proceed with pull and restart anyway? (y/N): ")
				var response string
				_, _ = fmt.Scanln(&response)

				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("❌ Update cancelled by user")
					return
				}
			} else {
				// Without -s the update narrows itself to what the table just
				// listed, so nothing the user saw as unchanged gets restarted.
				// An explicit -s still wins: naming services is a decision, and
				// this check is not entitled to overrule it.
				if len(scope) == 0 {
					scope = stale
					fmt.Printf("\n➡️  %d service(s) will be updated: %s\n",
						len(scope), strings.Join(scope, ", "))
					fmt.Println("   Everything else keeps running untouched.")
				}

				// Ask user for confirmation
				fmt.Print("Do you want to proceed with the update? (y/N): ")
				var response string
				_, _ = fmt.Scanln(&response)

				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("❌ Update cancelled by user")
					return
				}
			}

			//============================
			// Pull Images
			//============================
			fmt.Println("\n[Install the latest Docker images of the Cloud-Migrator subsystems.]")
			fmt.Println()

			if err := runCompose(append([]string{"pull", "--ignore-pull-failures"}, scope...)...); err != nil {
				fmt.Printf("❌ docker compose pull failed: %v\n", err)
				return
			}
		}

		//============================
		// RUN
		//============================
		fmt.Println("\n\n[Restart based on the installed latest Docker images.]")
		fmt.Println()

		// A scoped update brings the named services back up without the staged
		// OpenBao flow, so apply the same readiness check `infra run` uses. The
		// scope may come from -s or from the version check, and both need it.
		if len(scope) > 0 && !openBaoReadyForTargets(scope) {
			return
		}

		// Always bring the stack up detached, mirroring `infra run`. Running
		// attached ties the stack's lifetime to this terminal, so a Ctrl-C
		// meant to stop watching output tears the containers back down.
		if err := runCompose(append([]string{"up", "-d"}, scope...)...); err != nil {
			fmt.Printf("❌ docker compose up failed: %v\n", err)
			return
		}

		// Follow the logs unless the caller asked to stay detached.
		if !UpdateDetachMode {
			fmt.Println("\n[Showing container logs - Press Ctrl+C to stop viewing logs]")
			fmt.Println()
			if err := runCompose(append([]string{"logs", "-f"}, scope...)...); err != nil {
				fmt.Printf("❌ docker compose logs failed: %v\n", err)
			}
		} else {
			fmt.Println("\n📋 Log Commands:")
			fmt.Println("  ./mayfly infra logs                    # View all service logs")
			fmt.Println("  ./mayfly infra logs -s <service-name>  # View specific service logs")
			fmt.Println("  ./mayfly infra info                    # Check system status")
			fmt.Println()
		}
	},
}

// UpdateDetachMode keeps `infra update` from following logs after the stack is
// up. It is registered on updateCmd because `-d` used to be declared only on
// runCmd: `infra update -d` was rejected as an unknown flag while this code
// still read the run command's variable, which was therefore always false.
var UpdateDetachMode bool

func init() {
	dockerCmd.AddCommand(updateCmd)

	updateCmd.Flags().BoolVarP(&UpdateDetachMode, "detach", "d", false, "Detached mode: Run containers in the background without showing logs")
}
