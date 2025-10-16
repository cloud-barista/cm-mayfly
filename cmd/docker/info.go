package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"regexp"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// infoAllFlag represents the --all flag for showing all containers including stopped ones
var infoAllFlag bool

// infoHumanFlag represents the --human flag for showing human-readable service status
var infoHumanFlag bool

// infoTestVersionsFlag represents the --test-versions flag for debugging version extraction
var infoTestVersionsFlag bool

// infoCmd represents the info command.
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get information of Cloud-Migrator System",
	Long:  `Get information of Cloud-Migrator System. Information about containers and container images`,
	Run: func(cmd *cobra.Command, args []string) {
		if infoTestVersionsFlag {
			showVersionTestInfo()
		} else if infoHumanFlag {
			showHumanReadableInfo()
		} else {
			fmt.Println("\n[Get info for Cloud-Migrator runtimes]")
			fmt.Println()
			SysCallDockerComposePsWithAll(infoAllFlag)
			fmt.Println("")

			fmt.Println("[v]Status of Cloud-Migrator runtime images")
			convertedServiceName := convertServiceNameForDockerCompose(ServiceName)
			cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s images %s", ProjectName, DockerFilePath, convertedServiceName)
			//fmt.Println(cmdStr)
			common.SysCall(cmdStr)

			// Add helpful hint about --human option
			fmt.Println()
			fmt.Println("💡 Tip: Use '--human' or '-u' option for a more readable service status table:")
			fmt.Println("   ./mayfly infra info --human")
		}
	},
}

func init() {
	dockerCmd.AddCommand(infoCmd)

	// Add --all flag to info command
	infoCmd.Flags().BoolVarP(&infoAllFlag, "all", "a", false, "Show all containers including stopped ones")
	// Add --human flag to info command
	infoCmd.Flags().BoolVarP(&infoHumanFlag, "human", "u", false, "Show human-readable service status table")
	// Add --test-versions flag to info command
	infoCmd.Flags().BoolVarP(&infoTestVersionsFlag, "test-versions", "t", false, "Test version extraction from docker-compose.yaml and show service status")
}

// HumanServiceInfo represents service information for human-readable display
type HumanServiceInfo struct {
	Service      string
	Version      string
	Status       string
	Healthy      string
	InternalPort string
	ExternalPort string
	ImageSize    string
}

// showHumanReadableInfo displays service information in a human-readable table format
func showHumanReadableInfo() {
	fmt.Println("\n[Cloud-Migrator Service Status]")
	fmt.Println()

	// Get services from docker-compose.yaml
	allServices := getServicesFromCompose()

	// Filter services if -s option is used
	var services []string
	if ServiceName != "" {
		// Parse multiple services - support both comma and space separation
		var requestedServices []string
		if strings.Contains(ServiceName, ",") {
			// Comma-separated services
			requestedServices = strings.Split(ServiceName, ",")
		} else {
			// Space-separated services
			requestedServices = strings.Fields(ServiceName)
		}

		for _, requestedService := range requestedServices {
			requestedService = strings.TrimSpace(requestedService)

			// Check if the specified service exists
			found := false
			for _, service := range allServices {
				if service == requestedService {
					services = append(services, service)
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("❌ Service '%s' not found in docker-compose.yaml\n", requestedService)
				fmt.Println("Available services:")
				for _, service := range allServices {
					fmt.Printf("  - %s\n", service)
				}
				return
			}
		}

		// Add dependency services
		dependencyServices := getDependencyServices(services)
		services = append(services, dependencyServices...)

		// Remove duplicates
		services = removeDuplicateServices(services)
	} else {
		services = allServices
	}

	// Get container information - always show all containers for human-readable format
	containers := getContainerInfo(true)

	// Get image information
	images := getImageInfo()

	// Create service info table
	var serviceInfos []HumanServiceInfo

	for _, service := range services {
		info := HumanServiceInfo{
			Service: service,
		}

		// Get container info for this service
		if container, exists := containers[service]; exists {
			info.Status = container.Status
			info.Healthy = container.Healthy
			info.InternalPort = container.InternalPort
			info.ExternalPort = container.ExternalPort
			info.Version = container.Version
		} else {
			info.Status = "Not Found"
			info.Healthy = "-"
			info.InternalPort = "-"
			info.ExternalPort = "-"
			// For non-running services, get version from docker-compose.yaml with "(?)" suffix
			version := getVersionFromComposeFile(service)
			if version != "-" {
				info.Version = version + " (?)"
			} else {
				info.Version = version
			}
		}

		// Get image size - prioritize running container's image
		if container, exists := containers[service]; exists && container.Status == "running" {
			// For running containers, try to get the actual image size
			if image, exists := images[service]; exists {
				info.ImageSize = image.Size
			} else {
				info.ImageSize = "Running"
			}
		} else {
			// For non-running services, check if expected version image exists
			if image, exists := images[service]; exists {
				info.ImageSize = image.Size
			} else {
				info.ImageSize = "Not Downloaded"
			}
		}

		serviceInfos = append(serviceInfos, info)
	}

	// Display table with service categorization
	displayServiceTableWithDependencies(serviceInfos, ServiceName)
}

// ContainerInfo represents container information
type ContainerInfo struct {
	Status       string
	Healthy      string
	InternalPort string
	ExternalPort string
	Version      string
	Image        string
}

// ImageInfo represents image information
type ImageInfo struct {
	Size string
	Tag  string
}

// getServicesFromCompose extracts service names from docker-compose.yaml
func getServicesFromCompose() []string {
	// For now, return hardcoded services from docker-compose.yaml
	// In a real implementation, you would parse the YAML file
	return []string{
		"cb-spider",
		"cb-tumblebug",
		"cb-tumblebug-etcd",
		"cb-tumblebug-postgres",
		"cb-mapui",
		"cm-beetle",
		"cm-butterfly-api",
		"cm-butterfly-front",
		"cm-butterfly-db",
		"cm-honeybee",
		"cm-damselfly",
		"cm-cicada",
		"airflow-redis",
		"airflow-mysql",
		"airflow-server",
		"cm-grasshopper",
		"cm-ant",
		"ant-postgres",
	}
}

// getContainerInfo gets container information using docker compose ps
func getContainerInfo(showAll bool) map[string]ContainerInfo {
	containers := make(map[string]ContainerInfo)

	// Execute docker compose ps command
	var cmdStr string
	if showAll {
		cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s ps -a --format json", ProjectName, DockerFilePath)
	} else {
		cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s ps --format json", ProjectName, DockerFilePath)
	}
	cmd := exec.Command("bash", "-c", cmdStr)
	output, err := cmd.Output()
	if err != nil {
		return containers
	}

	// Parse JSON output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var container struct {
			Name    string `json:"Name"`
			Service string `json:"Service"`
			State   string `json:"State"`
			Status  string `json:"Status"`
			Health  string `json:"Health"`
			Ports   string `json:"Ports"`
			Image   string `json:"Image"`
		}

		if err := json.Unmarshal([]byte(line), &container); err != nil {
			continue
		}

		// Parse ports
		internalPort, externalPort := parsePorts(container.Ports)

		// Determine health status
		healthy := "-"
		if container.Health != "" {
			// Use Health field if available (more accurate)
			if container.Health == "healthy" {
				healthy = "✓"
			} else if container.Health == "unhealthy" {
				healthy = "✗"
			} else if container.Health == "starting" {
				healthy = "starting"
			}
		} else {
			// Fallback to Status field parsing
			if strings.Contains(container.Status, "healthy") {
				healthy = "✓"
			} else if strings.Contains(container.Status, "unhealthy") {
				healthy = "✗"
			}
		}

		// Extract version from image tag if available, otherwise use fallback
		version := extractVersionFromImage(container.Image)
		if version == "" {
			// If running container has no version tag (e.g., image ID), get from docker-compose.yaml
			version = getVersionFromComposeFile(container.Service)
		}

		// Normalize status display
		status := container.State
		if container.State == "exited" {
			status = "Stopped"
		}

		containers[container.Service] = ContainerInfo{
			Status:       status,
			Healthy:      healthy,
			InternalPort: internalPort,
			ExternalPort: externalPort,
			Version:      version,
			Image:        container.Image,
		}
	}

	return containers
}

// extractVersionFromImage extracts version/tag from image name
func extractVersionFromImage(imageName string) string {
	if imageName == "" {
		return ""
	}

	// Check if imageName is a SHA256 image ID (sha256:64-character-hex)
	if strings.HasPrefix(imageName, "sha256:") {
		hash := strings.TrimPrefix(imageName, "sha256:")
		if len(hash) == 64 && isHexString(hash) {
			return "" // Return empty to trigger fallback to getVersionFromComposeFile
		}
	}

	// Check if imageName is a short image ID (12 character hex string)
	if len(imageName) == 12 && isHexString(imageName) {
		return "" // Return empty to trigger fallback to getVersionFromComposeFile
	}

	// Split by colon to get tag part
	parts := strings.Split(imageName, ":")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}

	return ""
}

// isHexString checks if a string contains only hexadecimal characters
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// getImageInfo gets image information using docker images
func getImageInfo() map[string]ImageInfo {
	images := make(map[string]ImageInfo)

	// Execute docker images command to get all images
	cmdStr := "docker images --format json"
	cmd := exec.Command("bash", "-c", cmdStr)
	output, err := cmd.Output()
	if err != nil {
		return images
	}

	// Parse JSON output
	lines := strings.Split(string(output), "\n")
	allImages := make(map[string][]ImageInfo)

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var image struct {
			Repository string `json:"Repository"`
			Tag        string `json:"Tag"`
			Size       string `json:"Size"`
		}

		if err := json.Unmarshal([]byte(line), &image); err != nil {
			continue
		}

		// Match images to services based on repository name
		for serviceName, expectedImage := range getServiceImageMapping() {
			if strings.Contains(image.Repository, expectedImage) {
				if allImages[serviceName] == nil {
					allImages[serviceName] = []ImageInfo{}
				}
				allImages[serviceName] = append(allImages[serviceName], ImageInfo{
					Size: image.Size,
					Tag:  image.Tag,
				})
			}
		}
	}

	// For each service, select the appropriate image
	for serviceName, imageList := range allImages {
		if len(imageList) > 0 {
			// If multiple images exist, prefer the one matching docker-compose.yaml version
			expectedVersion := getVersionFromComposeFile(serviceName)
			selectedImage := imageList[0] // default to first

			for _, img := range imageList {
				if img.Tag == expectedVersion {
					selectedImage = img
					break
				}
			}

			images[serviceName] = selectedImage
		}
	}

	return images
}

// getServiceImageMapping returns mapping of service names to their image repository names
func getServiceImageMapping() map[string]string {
	return map[string]string{
		"cb-spider":             "cb-spider",
		"cb-tumblebug":          "cb-tumblebug",
		"cb-tumblebug-etcd":     "etcd",
		"cb-tumblebug-postgres": "postgres",
		"cb-mapui":              "cb-mapui",
		"cm-beetle":             "cm-beetle",
		"cm-butterfly-api":      "cm-butterfly-api",
		"cm-butterfly-front":    "cm-butterfly-front",
		"cm-butterfly-db":       "postgres",
		"cm-honeybee":           "cm-honeybee",
		"cm-damselfly":          "cm-damselfly",
		"cm-cicada":             "cm-cicada",
		"airflow-redis":         "redis",
		"airflow-mysql":         "mysql",
		"airflow-server":        "airflow-server",
		"cm-grasshopper":        "cm-grasshopper",
		"cm-ant":                "cm-ant",
		"ant-postgres":          "timescaledb",
	}
}

// getVersionFromComposeFile reads docker-compose.yaml and returns version for non-running services
func getVersionFromComposeFile(serviceName string) string {
	// Read docker-compose.yaml file
	composeFile := fmt.Sprintf("%s/docker-compose.yaml", filepath.Dir(DockerFilePath))
	content, err := os.ReadFile(composeFile)
	if err != nil {
		return "-"
	}

	// Use regex to find the service and its image
	// Pattern: serviceName: at the beginning of a line (with minimal indentation)
	servicePattern := fmt.Sprintf(`^\s*%s:\s*$`, regexp.QuoteMeta(serviceName))
	imagePattern := `^\s*image:\s*(.+)$`

	serviceRegex, err := regexp.Compile(servicePattern)
	if err != nil {
		return "-"
	}

	imageRegex, err := regexp.Compile(imagePattern)
	if err != nil {
		return "-"
	}

	lines := strings.Split(string(content), "\n")
	inService := false
	indentLevel := 0

	for i, line := range lines {
		// Check if we're entering the service section
		if serviceRegex.MatchString(line) {
			// Check if this is a top-level service (not inside depends_on or other sections)
			// by checking if the line starts with minimal indentation and is not inside depends_on
			trimmedLine := strings.TrimLeft(line, " \t")
			if strings.HasPrefix(trimmedLine, serviceName+":") {
				// Check if we're not inside depends_on section by looking at previous lines
				isInDependsOn := false
				for j := i - 1; j >= 0 && j >= i-10; j-- {
					prevLine := strings.TrimSpace(lines[j])
					if prevLine == "depends_on:" {
						isInDependsOn = true
						break
					}
					if prevLine != "" && !strings.HasPrefix(lines[j], " ") && !strings.HasPrefix(lines[j], "\t") {
						break
					}
				}

				if !isInDependsOn {
					inService = true
					// Calculate the indentation level of the service
					indentLevel = len(line) - len(strings.TrimLeft(line, " \t"))
					continue
				}
			}
		}

		// If we're in the service section, look for image field
		if inService {
			currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))

			// Check if we've moved to another service or section (same or less indentation)
			if strings.TrimSpace(line) != "" && currentIndent <= indentLevel && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				// We've moved to another service or section
				break
			}

			// Look for image field
			if matches := imageRegex.FindStringSubmatch(line); matches != nil {
				image := strings.TrimSpace(matches[1])

				// Extract version from image
				version := extractVersionFromImage(image)
				if version == "" {
					return "-"
				}

				// Check if image exists locally
				if !checkImageExists(image) {
					return version + " (Not Downloaded)"
				}

				return version
			}
		}
	}

	return "-"
}

// showVersionTestInfo displays version extraction test results and service status
func showVersionTestInfo() {
	fmt.Println("\n=== Version Extraction Test & Service Status ===")
	fmt.Println()

	// Get services from docker-compose.yaml
	allServices := getServicesFromCompose()

	// Get running containers
	containers := getContainerInfo(false)

	// Get image information
	images := getImageInfo()

	fmt.Printf("%-20s %-15s %-12s %-15s %s\n", "SERVICE", "COMPOSE_VERSION", "STATUS", "ACTUAL_VERSION", "IMAGE_SIZE")
	fmt.Println(strings.Repeat("-", 80))

	for _, service := range allServices {
		// Get version from docker-compose.yaml
		composeVersion := getVersionFromComposeFile(service)

		// Get actual running status and version
		var status, actualVersion, imageSize string

		if container, exists := containers[service]; exists {
			status = container.Status
			actualVersion = container.Version

			// Get image size
			if image, exists := images[service]; exists {
				imageSize = image.Size
			} else {
				imageSize = "-"
			}
		} else {
			status = "Not Running"
			actualVersion = "-"
			imageSize = "-"
		}

		fmt.Printf("%-20s %-15s %-12s %-15s %s\n",
			service, composeVersion, status, actualVersion, imageSize)
	}

	fmt.Println()
	fmt.Println("Legend:")
	fmt.Println("  COMPOSE_VERSION: Version specified in docker-compose.yaml")
	fmt.Println("  STATUS: Current container status (running/stopped/Not Running)")
	fmt.Println("  ACTUAL_VERSION: Version from running container image tag")
	fmt.Println("  IMAGE_SIZE: Size of the container image")
	fmt.Println("===============================================")
}

// checkImageExists checks if the specified image exists locally
func checkImageExists(imageName string) bool {
	cmd := exec.Command("docker", "images", "--format", "{{.Repository}}:{{.Tag}}", imageName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if the image exists in the output
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == imageName {
			return true
		}
	}
	return false
}

// parsePorts parses port information from docker compose ps output
func parsePorts(ports string) (string, string) {
	if ports == "" {
		return "-", "-"
	}

	// Parse multiple port mappings (e.g., "0.0.0.0:1024->1024/tcp, :::1024->1024/tcp")
	portMappings := strings.Split(ports, ",")
	if len(portMappings) > 0 {
		// Take the first port mapping
		firstMapping := strings.TrimSpace(portMappings[0])

		// Extract external and internal ports
		if strings.Contains(firstMapping, "->") {
			parts := strings.Split(firstMapping, "->")
			if len(parts) == 2 {
				external := strings.TrimSpace(parts[0])
				internal := strings.TrimSpace(parts[1])

				// Extract just the port number from external (e.g., "0.0.0.0:1024" -> "1024")
				if strings.Contains(external, ":") {
					externalParts := strings.Split(external, ":")
					external = externalParts[len(externalParts)-1]
				}

				// Extract just the port number from internal (e.g., "1024/tcp" -> "1024")
				if strings.Contains(internal, "/") {
					internalParts := strings.Split(internal, "/")
					internal = internalParts[0]
				}

				return internal, external
			}
		}
	}

	return "-", "-"
}

// displayServiceTable displays the service information in a formatted table
func displayServiceTable(services []HumanServiceInfo) {
	// Calculate column widths based on content
	maxServiceLen := 20 // minimum width
	maxVersionLen := 12
	maxStatusLen := 12
	maxHealthyLen := 8
	maxInternalLen := 12
	maxExternalLen := 12
	maxImageSizeLen := 15

	// Find maximum lengths
	for _, service := range services {
		if getDisplayWidth(service.Service) > maxServiceLen {
			maxServiceLen = getDisplayWidth(service.Service)
		}
		if getDisplayWidth(service.Version) > maxVersionLen {
			maxVersionLen = getDisplayWidth(service.Version)
		}
		if getDisplayWidth(service.Status) > maxStatusLen {
			maxStatusLen = getDisplayWidth(service.Status)
		}
		if getDisplayWidth(service.Healthy) > maxHealthyLen {
			maxHealthyLen = getDisplayWidth(service.Healthy)
		}
		if getDisplayWidth(service.InternalPort) > maxInternalLen {
			maxInternalLen = getDisplayWidth(service.InternalPort)
		}
		if getDisplayWidth(service.ExternalPort) > maxExternalLen {
			maxExternalLen = getDisplayWidth(service.ExternalPort)
		}
		if getDisplayWidth(service.ImageSize) > maxImageSizeLen {
			maxImageSizeLen = getDisplayWidth(service.ImageSize)
		}
	}

	// Add some padding
	maxServiceLen += 2
	maxVersionLen += 2
	maxStatusLen += 2
	maxHealthyLen += 2
	maxInternalLen += 2
	maxExternalLen += 2
	maxImageSizeLen += 2

	// Top border
	fmt.Printf("┌%s┬%s┬%s┬%s┬%s┬%s┬%s┐\n",
		strings.Repeat("─", maxServiceLen),
		strings.Repeat("─", maxVersionLen),
		strings.Repeat("─", maxStatusLen),
		strings.Repeat("─", maxHealthyLen),
		strings.Repeat("─", maxInternalLen),
		strings.Repeat("─", maxExternalLen),
		strings.Repeat("─", maxImageSizeLen))

	// Table header
	fmt.Printf("│%-*s│%-*s│%-*s│%-*s│%-*s│%-*s│%-*s│\n",
		maxServiceLen, "SERVICE",
		maxVersionLen, "VERSION",
		maxStatusLen, "STATUS",
		maxHealthyLen, "HEALTHY",
		maxInternalLen, "INTERNAL",
		maxExternalLen, "EXTERNAL",
		maxImageSizeLen, "IMAGE SIZE")

	// Header separator
	fmt.Printf("├%s┼%s┼%s┼%s┼%s┼%s┼%s┤\n",
		strings.Repeat("─", maxServiceLen),
		strings.Repeat("─", maxVersionLen),
		strings.Repeat("─", maxStatusLen),
		strings.Repeat("─", maxHealthyLen),
		strings.Repeat("─", maxInternalLen),
		strings.Repeat("─", maxExternalLen),
		strings.Repeat("─", maxImageSizeLen))

	// Table rows
	for _, service := range services {
		fmt.Printf("│%-*s│%-*s│%-*s│%-*s│%-*s│%-*s│%-*s│\n",
			maxServiceLen, service.Service,
			maxVersionLen, service.Version,
			maxStatusLen, service.Status,
			maxHealthyLen, service.Healthy,
			maxInternalLen, service.InternalPort,
			maxExternalLen, service.ExternalPort,
			maxImageSizeLen, service.ImageSize)
	}

	// Bottom border
	fmt.Printf("└%s┴%s┴%s┴%s┴%s┴%s┴%s┘\n",
		strings.Repeat("─", maxServiceLen),
		strings.Repeat("─", maxVersionLen),
		strings.Repeat("─", maxStatusLen),
		strings.Repeat("─", maxHealthyLen),
		strings.Repeat("─", maxInternalLen),
		strings.Repeat("─", maxExternalLen),
		strings.Repeat("─", maxImageSizeLen))

	fmt.Println()
}

// getDisplayWidth calculates the display width of a string, accounting for Unicode characters
func getDisplayWidth(s string) int {
	width := 0
	for _, r := range s {
		// Handle specific Unicode characters that might have different display widths
		switch r {
		case '✓': // check mark
			width += 1
		case '✗': // cross mark
			width += 1
		case '-': // dash
			width += 1
		default:
			if r < 0x80 {
				width++ // ASCII
			} else {
				width += 1 // Most Unicode characters are 1 width
			}
		}
	}
	return width
}

// displayServiceTableWithDependencies displays the service information with dependency categorization
func displayServiceTableWithDependencies(services []HumanServiceInfo, requestedServices string) {
	if requestedServices == "" {
		// No specific service requested, show all services in one table
		displayServiceTable(services)
		return
	}

	// Parse requested services - support both comma and space separation
	var requestedList []string
	if strings.Contains(requestedServices, ",") {
		// Comma-separated services
		requestedList = strings.Split(requestedServices, ",")
	} else {
		// Space-separated services
		requestedList = strings.Fields(requestedServices)
	}

	for i, service := range requestedList {
		requestedList[i] = strings.TrimSpace(service)
	}

	// Get dependency services (for reference, not used in categorization logic)
	_ = getDependencyServices(requestedList)

	// Categorize services
	var mainServices []HumanServiceInfo
	var depServices []HumanServiceInfo

	for _, service := range services {
		isMainService := false
		for _, reqService := range requestedList {
			if service.Service == reqService {
				isMainService = true
				break
			}
		}

		if isMainService {
			mainServices = append(mainServices, service)
		} else {
			depServices = append(depServices, service)
		}
	}

	// Display main services
	if len(mainServices) > 0 {
		fmt.Println("🎯 Requested Services:")
		displayServiceTable(mainServices)
	}

	// Display dependency services
	if len(depServices) > 0 {
		fmt.Println("📦 Dependency Services:")
		displayServiceTable(depServices)
	}
}

// getDependencyServices returns dependency services for the given services
func getDependencyServices(services []string) []string {
	var dependencies []string

	// Define service dependencies based on docker-compose.yaml
	serviceDependencies := map[string][]string{
		"cb-tumblebug":       {"cb-tumblebug-etcd", "cb-spider", "cb-tumblebug-postgres"},
		"cm-beetle":          {"cb-tumblebug"},
		"cm-butterfly-api":   {"cm-butterfly-db"},
		"cm-butterfly-front": {"cm-butterfly-api"},
		"cm-cicada":          {"cm-damselfly", "cm-beetle", "cm-grasshopper", "airflow-server"},
		"cm-grasshopper":     {"cm-honeybee"},
		"cm-ant":             {"cb-tumblebug", "ant-postgres"},
		"airflow-server":     {"airflow-mysql", "airflow-redis"},
	}

	for _, service := range services {
		if deps, exists := serviceDependencies[service]; exists {
			dependencies = append(dependencies, deps...)
		}
	}

	return dependencies
}

// removeDuplicateServices removes duplicate services from the slice
func removeDuplicateServices(services []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, service := range services {
		if !keys[service] {
			keys[service] = true
			result = append(result, service)
		}
	}

	return result
}
