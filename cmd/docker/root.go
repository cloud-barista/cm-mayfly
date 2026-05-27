/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cm-mayfly/cm-mayfly/cmd"
	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// DockerFilePath is a variable that holds path to the docker-compose.yaml.
var DockerFilePath string

// ProjectName is a variable that holds docker compose project name.
var ProjectName string

// ServiceName is used when you want to specify only a specific service
var ServiceName string

// restCmd represents the rest command
var dockerCmd = &cobra.Command{
	Use:   "infra",
	Short: "Installing and managing cloud-migrator's infrastructure",
	Long: `Build the environment of the infrastructure required for cloud-migrator and monitor the running status of the infrastructure.
For example, you can install and run, stop, update and ... Cloud-Migrator runtimes.

- ./mayfly infra install
- ./mayfly infra run
- ./mayfly infra run -d
- ./mayfly infra run -d -s cb-tumblebug
- ./mayfly infra info
- ./mayfly infra stop
- ./mayfly infra update
- ./mayfly infra remove -v -i
	     `,
	Run: func(cmd *cobra.Command, args []string) {
		//fmt.Println(cmd.UsageString())
		//fmt.Println(cmd.Help())
		cmd.Help()
	},
	// Before any docker (infra) subcommand runs a `docker compose` command,
	// make sure the shared environment file exists. The compose file relies on
	// it for ${VAR} interpolation, so running without it would fail with
	// confusing "variable is not set" warnings.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// The bare `infra` command only prints help and does not invoke
		// docker compose, so it does not require the .env file.
		if !cmd.HasParent() || cmd.Name() == "infra" {
			return nil
		}
		return ensureDockerEnvFile()
	},
}

// ensureDockerEnvFile verifies that the docker-compose environment file exists
// next to the compose file (DockerFilePath). It returns an error with English
// guidance if the file is missing, so the docker compose command is never run
// with unset variables.
func ensureDockerEnvFile() error {
	dir := filepath.Dir(DockerFilePath)
	envPath := filepath.Join(dir, ".env")
	examplePath := filepath.Join(dir, ".env.example")
	if _, err := os.Stat(envPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("environment file not found: %s\n\n"+
				"docker-compose.yaml requires this file for ${VAR} interpolation "+
				"(DB credentials, SMTP, log levels, etc.).\n"+
				"Create it from the template, then edit the values before running this command again:\n\n"+
				"  cp %s %s\n",
				envPath, examplePath, envPath)
		}
		return fmt.Errorf("failed to check environment file %s: %w", envPath, err)
	}
	return nil
}

// convertServiceNameForDockerCompose converts comma-separated service names to space-separated
// for Docker Compose command compatibility
func convertServiceNameForDockerCompose(serviceName string) string {
	if serviceName == "" {
		return ""
	}

	// If contains comma, convert to space-separated
	if strings.Contains(serviceName, ",") {
		services := strings.Split(serviceName, ",")
		var result []string
		for _, service := range services {
			trimmed := strings.TrimSpace(service)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return strings.Join(result, " ")
	}

	// If space-separated or single service, return as is
	return serviceName
}

// SysCallDockerComposePsWithAll executes `docker-compose ps` command with optional --all flag
func SysCallDockerComposePsWithAll(showAll bool) {
	fmt.Println("\n[v]Status of Cloud-Migrator runtimes")
	var cmdStr string
	convertedServiceName := convertServiceNameForDockerCompose(ServiceName)
	if showAll {
		cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s ps -a %s", ProjectName, DockerFilePath, convertedServiceName)
	} else {
		cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s ps %s", ProjectName, DockerFilePath, convertedServiceName)
	}
	common.SysCall(cmdStr)
}

func init() {
	cmd.RootCmd.AddCommand(dockerCmd)

	// Add flags for Docker Compose yaml File Path
	dockerCmd.PersistentFlags().StringVarP(&DockerFilePath, "file", "f", common.DefaultDockerComposeConfig, "User-defined configuration file")

	// Add flags for Docker Compose project name
	dockerCmd.PersistentFlags().StringVarP(&ProjectName, "project-name", "p", common.ComposeProjectName, "User-defined docker compose porject name")

	// ServiceName is used when you want to specify only a specific service
	dockerCmd.PersistentFlags().StringVarP(&ServiceName, "service", "s", "", "Want to target specific services only(Default : all)")
}
