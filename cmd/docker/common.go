package docker

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ServiceInfo represents service information with category
type ServiceInfo struct {
	Name     string
	Image    string
	Category string
}

// parseDockerComposeImages parses docker-compose.yaml to extract all service information
func parseDockerComposeImages() (map[string]ServiceInfo, error) {
	services := make(map[string]ServiceInfo)

	// Read docker-compose.yaml file
	content, err := os.ReadFile(DockerFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read docker-compose.yaml: %v", err)
	}

	// Regex to match service names and image definitions
	serviceRegex := regexp.MustCompile(`^\s*([a-zA-Z0-9_-]+):\s*$`)
	imageRegex := regexp.MustCompile(`^\s*image:\s*([^:\s]+):([^\s]+)\s*$`)

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
			fullImage := fmt.Sprintf("%s:%s", imageName, tag)

			// Categorize services
			category := categorizeService(currentService, imageName)

			services[currentService] = ServiceInfo{
				Name:     currentService,
				Image:    fullImage,
				Category: category,
			}
			currentService = ""
		}
	}

	return services, nil
}

// categorizeService categorizes services based on name and image
func categorizeService(serviceName, imageName string) string {
	// Core Cloud Migrator services
	if strings.HasPrefix(imageName, "cloudbaristaorg/") {
		return "Core Services"
	}

	// Database services
	if strings.Contains(serviceName, "postgres") || strings.Contains(serviceName, "mysql") ||
		strings.Contains(serviceName, "db") {
		return "Database"
	}

	// Cache/Message Queue services
	if strings.Contains(serviceName, "redis") || strings.Contains(serviceName, "etcd") {
		return "Cache/Storage"
	}

	// Airflow services
	if strings.Contains(serviceName, "airflow") {
		return "Workflow Engine"
	}

	// Default category
	return "Dependencies"
}
