/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package docker

import (
	"fmt"

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
}

// SysCallDockerComposePs executes `docker-compose ps` command via system call.
func SysCallDockerComposePs() {
	fmt.Println("\n[v]Status of Cloud-Migrator runtimes")
	cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s ps", ProjectName, DockerFilePath)
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
