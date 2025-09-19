package setup

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// tumblebugInitCmd represents the tumblebug-init command
var tumblebugInitCmd = &cobra.Command{
	Use:   "tumblebug-init",
	Short: "Initialize CB-Tumblebug with the current running version",
	Long: `Initialize CB-Tumblebug with the current running version.
This command will:
1. Check if CB-Tumblebug is running
2. Check the current running CB-Tumblebug version
3. Download the corresponding version from GitHub
4. Execute the initialization script

Before running this command, you need to create encrypted credential files.
Please refer to: https://github.com/cloud-barista/cb-tumblebug?tab=readme-ov-file#installation--setup-`,
	Run: func(cmd *cobra.Command, args []string) {
		runTumblebugInit()
	},
}

func init() {
	setupCmd.AddCommand(tumblebugInitCmd)
}

// runTumblebugInit executes the tumblebug initialization process
func runTumblebugInit() {
	fmt.Println("\n[CB-Tumblebug Initialization]")

	// Store current working directory
	originalDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		return
	}

	// Check if CB-Tumblebug is running
	if !isTumblebugRunning() {
		fmt.Println("❌ CB-Tumblebug is not running.")
		fmt.Println("Please start the Cloud-Migrator system first:")
		fmt.Println("   ./mayfly infra run")
		fmt.Println()
		fmt.Println("Please try again after the system is running.")
		return
	}

	fmt.Println("✅ CB-Tumblebug is running.")
	fmt.Println("Checking Tumblebug execution version...")

	// Get current running CB-Tumblebug version
	version, err := getCurrentTumblebugVersion()
	if err != nil {
		fmt.Printf("Error getting current Tumblebug version: %v\n", err)
		return
	}

	gitTag := "v" + version
	fmt.Printf("✅ Version confirmed: %s\n", version)

	// Show warning message about credential files
	showCredentialWarning(gitTag)

	// Ask for user confirmation
	if !askForConfirmation("Do you want to proceed with Tumblebug Init using prepared encrypted credentials?") {
		fmt.Println("Operation cancelled.")
		return
	}

	// Download and initialize Tumblebug
	err = downloadAndInitTumblebug(version, originalDir)
	if err != nil {
		fmt.Printf("Error during Tumblebug initialization: %v\n", err)
		// Return to original directory even on error
		os.Chdir(originalDir)
		return
	}

	// Return to original directory
	err = os.Chdir(originalDir)
	if err != nil {
		fmt.Printf("Warning: Could not return to original directory: %v\n", err)
	} else {
		fmt.Printf("\nReturned to original location: %s\n", originalDir)
	}

	fmt.Println("\nCB-Tumblebug initialization completed.")
}

// isTumblebugRunning checks if CB-Tumblebug container is running
func isTumblebugRunning() bool {
	cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s ps --format json", common.ComposeProjectName, common.DefaultDockerComposeConfig)
	cmd := exec.Command("/bin/sh", "-c", cmdStr)

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if cb-tumblebug service is running
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "cb-tumblebug") && strings.Contains(line, "running") {
			return true
		}
	}

	return false
}

// showCredentialWarning displays warning about credential files
func showCredentialWarning(gitTag string) {
	cloneCmd := fmt.Sprintf("git clone -b %s https://github.com/cloud-barista/cb-tumblebug.git", gitTag)
	fmt.Printf("\n[ Important Notice ]\n")
	fmt.Printf("Tumblebug %s version is running.\n", gitTag)
	fmt.Println("Encrypted credential files must be prepared before running Tumblebug initialization.")
	fmt.Println("If encrypted credential files are not available, please create them first by referring to the guide below.")
	fmt.Printf("   Guide: https://github.com/cloud-barista/cb-tumblebug/tree/%s?tab=readme-ov-file#installation--setup-\n", gitTag)
	fmt.Printf("   Download: %s\n", cloneCmd)

	fmt.Println()
}

// askForConfirmation asks user for confirmation
func askForConfirmation(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s (y/N): ", message)
		response, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			return false
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" || response == "" {
			return false
		}
		fmt.Println("Please enter 'y' for yes or 'n' for no.")
	}
}

// getCurrentTumblebugVersion gets the current running CB-Tumblebug version
func getCurrentTumblebugVersion() (string, error) {
	// First try to get version from docker compose ps
	version, err := getVersionFromDockerCompose()
	if err == nil && version != "" {
		return version, nil
	}

	// Fallback to docker-compose.yaml file
	version, err = getVersionFromDockerComposeFile()
	if err != nil {
		return "", fmt.Errorf("could not determine Tumblebug version: %v", err)
	}

	return version, nil
}

// getVersionFromDockerCompose gets version from running docker compose ps
func getVersionFromDockerCompose() (string, error) {
	cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s ps --format json", common.ComposeProjectName, common.DefaultDockerComposeConfig)
	cmd := exec.Command("/bin/sh", "-c", cmdStr)

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse JSON output to find cb-tumblebug service
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "cb-tumblebug") && strings.Contains(line, "cloudbaristaorg/cb-tumblebug:") {
			// Extract version from image name
			re := regexp.MustCompile(`cloudbaristaorg/cb-tumblebug:([0-9]+\.[0-9]+\.[0-9]+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				return matches[1], nil
			}
		}
	}

	return "", fmt.Errorf("version not found in docker compose ps output")
}

// getVersionFromDockerComposeFile gets version from docker-compose.yaml file
func getVersionFromDockerComposeFile() (string, error) {
	file, err := os.Open(common.DefaultDockerComposeConfig)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "image: cloudbaristaorg/cb-tumblebug:") {
			// Extract version from image line
			re := regexp.MustCompile(`cloudbaristaorg/cb-tumblebug:([0-9]+\.[0-9]+\.[0-9]+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				return matches[1], nil
			}
		}
	}

	return "", fmt.Errorf("version not found in docker-compose.yaml")
}

// downloadAndInitTumblebug downloads and initializes Tumblebug
func downloadAndInitTumblebug(version, originalDir string) error {
	// Convert version to GitHub tag format (add 'v' prefix)
	gitTag := "v" + version

	// Create target directory
	targetDir := filepath.Join(os.Getenv("HOME"), "go", "src", "github.com", "cloud-barista")
	cbTumblebugDir := filepath.Join(targetDir, "cb-tumblebug")

	fmt.Printf("Downloading CB-Tumblebug %s version from GitHub...\n", gitTag)
	fmt.Printf("Target directory: %s\n", cbTumblebugDir)

	// Check if directory already exists
	if _, err := os.Stat(cbTumblebugDir); err == nil {
		return handleExistingDirectory(cbTumblebugDir, gitTag, targetDir, originalDir)
	}

	// Create directory structure
	err := os.MkdirAll(targetDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create target directory: %v", err)
	}

	// Change to target directory
	err = os.Chdir(targetDir)
	if err != nil {
		return fmt.Errorf("failed to change to target directory: %v", err)
	}

	// Clone the repository with specific tag
	cloneCmd := fmt.Sprintf("git clone -b %s https://github.com/cloud-barista/cb-tumblebug.git", gitTag)
	fmt.Printf("Executing command: %s\n", cloneCmd)

	err = common.SysCallWithError(cloneCmd)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %v", err)
	}

	// Initialize Tumblebug
	fmt.Printf("\nExecuting cb-tumblebug in %s folder...\n", cbTumblebugDir)
	return initializeTumblebug(cbTumblebugDir, originalDir)
}

// handleExistingDirectory handles the case when cb-tumblebug directory already exists
func handleExistingDirectory(cbTumblebugDir, gitTag, targetDir, originalDir string) error {
	fmt.Printf("Same version of Tumblebug already exists in %s folder.\n", cbTumblebugDir)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Do you want to delete existing files and download fresh, or use existing files without downloading? (d=delete and download, e=use existing): ")
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error reading input: %v", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		switch response {
		case "d":
			// Remove existing directory and download fresh
			err := os.RemoveAll(cbTumblebugDir)
			if err != nil {
				return fmt.Errorf("failed to remove existing directory: %v", err)
			}

			// Change to target directory
			err = os.Chdir(targetDir)
			if err != nil {
				return fmt.Errorf("failed to change to target directory: %v", err)
			}

			// Clone fresh copy
			cloneCmd := fmt.Sprintf("git clone -b %s https://github.com/cloud-barista/cb-tumblebug.git", gitTag)
			fmt.Printf("Executing command: %s\n", cloneCmd)

			err = common.SysCallWithError(cloneCmd)
			if err != nil {
				return fmt.Errorf("failed to clone repository: %v", err)
			}

			fmt.Printf("\nExecuting cb-tumblebug in %s folder...\n", cbTumblebugDir)
			return initializeTumblebug(cbTumblebugDir, originalDir)

		case "e":
			// Use existing directory
			fmt.Printf("\nExecuting cb-tumblebug in %s folder...\n", cbTumblebugDir)
			return initializeTumblebug(cbTumblebugDir, originalDir)

		default:
			fmt.Println("Please enter 'd' for delete and download, or 'e' for use existing.")
		}
	}
}

// initializeTumblebug initializes Tumblebug by running setup.env and init.sh
func initializeTumblebug(cbTumblebugDir, originalDir string) error {
	fmt.Printf("Starting CB-Tumblebug initialization: %s\n", cbTumblebugDir)

	// Create a script that will run in isolation
	script := fmt.Sprintf(`#!/bin/bash
set -e

# Change to cb-tumblebug directory
cd "%s"

echo "Current location: $(pwd)"

# Source setup.env if it exists
if [ -f "conf/setup.env" ]; then
    echo "Executing setup.env file..."
    source conf/setup.env
    echo "setup.env execution completed"
else
    echo "Warning: conf/setup.env file not found."
fi

# Run init.sh if it exists
if [ -f "init/init.sh" ]; then
    echo "Executing init.sh file..."
    chmod +x init/init.sh
    # Run init.sh with proper stdin/stdout/stderr handling
    ./init/init.sh
    echo "init.sh execution completed"
else
    echo "Error: init/init.sh file not found."
    exit 1
fi

echo "CB-Tumblebug initialization completed."
`, cbTumblebugDir)

	// Write script to temporary file
	tmpScript := filepath.Join(os.TempDir(), "tumblebug_init.sh")
	err := os.WriteFile(tmpScript, []byte(script), 0755)
	if err != nil {
		return fmt.Errorf("failed to create temporary script: %v", err)
	}
	defer os.Remove(tmpScript)

	// Execute the script in a new shell with proper stdin/stdout/stderr handling
	fmt.Println("Executing Tumblebug initialization in separate shell...")
	fmt.Println("Note: You will be prompted for user input during the initialization process.")

	cmd := exec.Command("/bin/bash", tmpScript)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin // This ensures stdin is properly connected

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute initialization script: %v", err)
	}

	return nil
}
