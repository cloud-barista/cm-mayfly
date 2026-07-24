package docker

import (
	"encoding/json"
	"fmt"
	"net/http"
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

// checkDockerHubTagUpdate checks if the specific tag on Docker Hub has been updated
func checkDockerHubTagUpdate(imageName, tag string) (string, error) {
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags/%s/",
		dockerHubRepositoryPath(imageName), tag)

	// The shared client carries a request timeout. The bare http.Get this
	// replaced used the default client, which has none, so a slow or
	// unresponsive Docker Hub left `infra update` waiting with no way out.
	resp, err := common.NewHTTPClient().R().Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Docker Hub API: %v", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("Docker Hub API returned status %d", resp.StatusCode())
	}

	var tagResponse DockerHubTagDetailResponse
	if err := json.Unmarshal(resp.Body(), &tagResponse); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return tagResponse.LastUpdated, nil
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

// checkVersionUpdates checks for version updates and displays comparison.
//
// Individual lookups stay best-effort — a service whose Docker Hub tag cannot be
// read still gets a row, with "-" in the Latest column, because the Local vs
// Compose comparison is what actually decides whether an update is needed and
// that comes from the local daemon. An error is returned only when the local
// lookup failed for every service examined: at that point nothing in the table
// is grounded in the real state of the host, so the returned hasUpdates says
// nothing, and the caller must not put a confirmation prompt in front of the
// user based on it.
//
// This function used to return a literal nil on every path, which made the
// caller's `if err != nil` fallback dead code and meant a completely unusable
// docker daemon still produced a confident-looking "All services are up to date!".
func checkVersionUpdates(services map[string]ServiceInfo) (bool, error) {
	fmt.Println("🔍 Checking version updates...")
	fmt.Println()

	hasUpdates := false
	updateInfo := make(map[string]map[string]string)

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

		// Check if the specific tag on Docker Hub has been updated
		dockerHubLastUpdated, err := checkDockerHubTagUpdate(imageName, composeTag)
		if err != nil {
			dockerHubLastUpdated = "unknown"
		}

		// Determine latest status
		var latestStatus string
		if dockerHubLastUpdated == "unknown" {
			latestStatus = "-"
		} else if currentVersion == "not_installed" {
			latestStatus = composeTag // Need to install
		} else if currentVersion == composeTag {
			latestStatus = "-" // Same version, no update needed
		} else {
			latestStatus = composeTag // Different version, need to update
		}

		// Store update information
		updateInfo[serviceName] = map[string]string{
			"local":   currentVersion,
			"compose": composeTag,
			"latest":  latestStatus,
		}

		// Check if updates are needed
		if currentVersion == "not_installed" || currentVersion != composeTag {
			hasUpdates = true
		}
	}

	// Every local lookup failed, so "Local" is "unknown" on every row and the
	// hasUpdates it produced is an artefact of that, not a finding. Report it
	// instead of letting the caller act on it.
	if examined > 0 && localFailures == examined {
		return false, fmt.Errorf("could not read the locally installed image version for any of the %d service(s); is the docker daemon reachable? (first failure — %w)", examined, firstLocalErr)
	}

	// Find the longest service name and version strings for proper table width
	maxServiceLen := 15 // minimum width
	maxVersionLen := 11 // minimum width for version columns

	for serviceName, versions := range updateInfo {
		if len(serviceName) > maxServiceLen {
			maxServiceLen = len(serviceName)
		}
		// Check all version strings
		for _, version := range versions {
			if len(version) > maxVersionLen {
				maxVersionLen = len(version)
			}
		}
	}

	// Display version comparison
	fmt.Println("📊 Version Comparison:")

	// Create table header with dynamic width
	headerFormat := fmt.Sprintf("┌─%%-%ds─┬─%%-%ds─┬─%%-%ds─┬─%%-%ds─┐\n", maxServiceLen, maxVersionLen, maxVersionLen, maxVersionLen)
	separatorFormat := fmt.Sprintf("├─%%-%ds─┼─%%-%ds─┼─%%-%ds─┼─%%-%ds─┤\n", maxServiceLen, maxVersionLen, maxVersionLen, maxVersionLen)
	footerFormat := fmt.Sprintf("└─%%-%ds─┴─%%-%ds─┴─%%-%ds─┴─%%-%ds─┘\n", maxServiceLen, maxVersionLen, maxVersionLen, maxVersionLen)

	// Print top border
	fmt.Printf(headerFormat, strings.Repeat("─", maxServiceLen), strings.Repeat("─", maxVersionLen), strings.Repeat("─", maxVersionLen), strings.Repeat("─", maxVersionLen))
	// Print header row
	fmt.Printf("│ %-*s │ %-*s │ %-*s │ %-*s │\n", maxServiceLen, "Service", maxVersionLen, "Local", maxVersionLen, "Compose", maxVersionLen, "Latest")
	// Print separator
	fmt.Printf(separatorFormat, strings.Repeat("─", maxServiceLen), strings.Repeat("─", maxVersionLen), strings.Repeat("─", maxVersionLen), strings.Repeat("─", maxVersionLen))

	for serviceName, versions := range updateInfo {
		local := versions["local"]
		compose := versions["compose"]
		latest := versions["latest"]

		// Determine status indicators using clear symbols
		var status string
		if local == "not_installed" {
			status = "✗" // Image not installed locally
		} else if local != compose {
			status = "●" // Local version differs from compose (need update)
		} else {
			status = "✓" // All versions match
		}

		fmt.Printf("│ %-*s │ %-*s │ %-*s │ %-*s │ %s\n",
			maxServiceLen, serviceName, maxVersionLen, local, maxVersionLen, compose, maxVersionLen, latest, status)
	}

	fmt.Printf(footerFormat, strings.Repeat("─", maxServiceLen), strings.Repeat("─", maxVersionLen), strings.Repeat("─", maxVersionLen), strings.Repeat("─", maxVersionLen))
	fmt.Println()
	fmt.Println("Legend:")
	fmt.Println("✓ All versions match")
	fmt.Println("● Local version differs from docker-compose.yaml (update needed)")
	fmt.Println("✗ Image not installed locally")
	fmt.Println()

	return hasUpdates, nil
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

		// Parse docker-compose.yaml to get actual image information
		services, err := parseDockerComposeImages()
		if err != nil {
			fmt.Printf("❌ Failed to parse docker-compose.yaml: %v\n", err)
			fmt.Printf("🔄 Falling back to regular pull...\n")

			// Fallback to regular pull without version check
			if err := runCompose(append([]string{"pull", "--ignore-pull-failures"}, targets...)...); err != nil {
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
			hasUpdates, err := checkVersionUpdates(services)
			if err != nil {
				fmt.Printf("⚠️ Failed to check version updates: %v\n", err)
				fmt.Printf("🔄 Proceeding with regular pull...\n")
			} else if !hasUpdates {
				fmt.Println("✅ All services are up to date!")
				fmt.Print("Do you want to proceed with pull and restart anyway? (y/N): ")
				var response string
				_, _ = fmt.Scanln(&response)

				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("❌ Update cancelled by user")
					return
				}
			} else {
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

			if err := runCompose(append([]string{"pull", "--ignore-pull-failures"}, targets...)...); err != nil {
				fmt.Printf("❌ docker compose pull failed: %v\n", err)
				return
			}
		}

		//============================
		// RUN
		//============================
		fmt.Println("\n\n[Restart based on the installed latest Docker images.]")
		fmt.Println()

		// A targeted update brings the named services back up without the staged
		// OpenBao flow, so apply the same readiness check `infra run` uses.
		if len(targets) > 0 && !openBaoReadyForTargets(targets) {
			return
		}

		// Always bring the stack up detached, mirroring `infra run`. Running
		// attached ties the stack's lifetime to this terminal, so a Ctrl-C
		// meant to stop watching output tears the containers back down.
		if err := runCompose(append([]string{"up", "-d"}, targets...)...); err != nil {
			fmt.Printf("❌ docker compose up failed: %v\n", err)
			return
		}

		// Follow the logs unless the caller asked to stay detached.
		if !UpdateDetachMode {
			fmt.Println("\n[Showing container logs - Press Ctrl+C to stop viewing logs]")
			fmt.Println()
			if err := runCompose(append([]string{"logs", "-f"}, targets...)...); err != nil {
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
