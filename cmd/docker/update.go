package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
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

// checkDockerHubTagUpdate checks if the specific tag on Docker Hub has been updated
func checkDockerHubTagUpdate(imageName, tag string) (string, error) {
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags/%s/", imageName, tag)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Docker Hub API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Docker Hub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	var tagResponse DockerHubTagDetailResponse
	if err := json.Unmarshal(body, &tagResponse); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return tagResponse.LastUpdated, nil
}

// getCurrentLocalVersion gets the current local image version of a service
func getCurrentLocalVersion(imageName, tag string) (string, error) {
	// Check if image exists locally
	cmdStr := fmt.Sprintf("docker images --format '{{.Tag}}' %s:%s 2>/dev/null || echo 'not_installed'", imageName, tag)
	output := common.SysCallWithOutput(cmdStr)

	if strings.TrimSpace(output) == "not_installed" {
		return "not_installed", nil
	}

	// Return the tag if image exists locally
	return strings.TrimSpace(output), nil
}

// parseDockerComposeImages parses docker-compose.yaml to extract image information
func parseDockerComposeImages() (map[string]string, error) {
	services := make(map[string]string)

	// Read docker-compose.yaml file
	content, err := os.ReadFile(DockerFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read docker-compose.yaml: %v", err)
	}

	// Regex to match service names and image definitions
	serviceRegex := regexp.MustCompile(`^\s*([a-zA-Z0-9_-]+):\s*$`)
	imageRegex := regexp.MustCompile(`^\s*image:\s*(cloudbaristaorg/[^:\s]+):([^\s]+)\s*$`)

	lines := strings.Split(string(content), "\n")
	var currentService string

	for _, line := range lines {
		// Check for service name (e.g., "  cm-ant:")
		if matches := serviceRegex.FindStringSubmatch(line); matches != nil {
			currentService = matches[1]
			continue
		}

		// Check for image definition (e.g., "    image: cloudbaristaorg/cm-ant:0.4.0")
		if matches := imageRegex.FindStringSubmatch(line); matches != nil && currentService != "" {
			imageName := matches[1]
			tag := matches[2]
			services[currentService] = fmt.Sprintf("%s:%s", imageName, tag)
			currentService = ""
		}
	}

	return services, nil
}

// checkVersionUpdates checks for version updates and displays comparison
func checkVersionUpdates(services map[string]string) (bool, error) {
	fmt.Println("🔍 Checking version updates...")
	fmt.Println()

	hasUpdates := false
	updateInfo := make(map[string]map[string]string)

	for serviceName, imageWithTag := range services {
		// Extract image name and tag
		parts := strings.Split(imageWithTag, ":")
		if len(parts) != 2 {
			continue
		}
		imageName := parts[0]
		composeTag := parts[1]

		// Get current local version
		currentVersion, err := getCurrentLocalVersion(imageName, composeTag)
		if err != nil {
			currentVersion = "unknown"
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

		// Parse docker-compose.yaml to get actual image information
		services, err := parseDockerComposeImages()
		if err != nil {
			fmt.Printf("❌ Failed to parse docker-compose.yaml: %v\n", err)
			fmt.Printf("🔄 Falling back to regular pull...\n")

			// Fallback to regular pull without version check
			cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s pull --ignore-pull-failures %s", ProjectName, DockerFilePath, ServiceName)
			common.SysCall(cmdStr)
		} else {
			// If specific service is requested, only check that service
			if ServiceName != "" {
				if imageName, exists := services[ServiceName]; exists {
					services = map[string]string{ServiceName: imageName}
				} else {
					fmt.Printf("⚠️ Service %s not found in docker-compose.yaml\n", ServiceName)
					return
				}
			}

			// Check for version updates
			hasUpdates, err := checkVersionUpdates(services)
			if err != nil {
				fmt.Printf("⚠️ Failed to check version updates: %v\n", err)
				fmt.Printf("🔄 Proceeding with regular pull...\n")
			} else if !hasUpdates {
				fmt.Println("✅ All services are up to date!")
				fmt.Println("🔄 Proceeding with restart...")
			} else {
				// Ask user for confirmation
				fmt.Print("Do you want to proceed with the update? (y/N): ")
				var response string
				fmt.Scanln(&response)

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

			cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s pull --ignore-pull-failures %s", ProjectName, DockerFilePath, ServiceName)
			common.SysCall(cmdStr)
		}

		//============================
		// RUN
		//============================
		fmt.Println("\n\n[Restart based on the installed latest Docker images.]")
		fmt.Println()

		detachModeOption := ""
		if DetachMode {
			detachModeOption = "-d"
		}
		cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s up %s %s", ProjectName, DockerFilePath, detachModeOption, ServiceName)

		common.SysCall(cmdStr)

	},
}

func init() {
	dockerCmd.AddCommand(updateCmd)
}
