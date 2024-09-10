package docker

import (
	"fmt"

	"github.com/cm-mayfly/cm-mayfly/common"
)

// DockerFilePath is a variable that holds path to the docker-compose.yaml.
var DockerFilePath string

const (

	// DefaultDockerComposeConfig is a variable that holds path to docker-compose.yaml
	//DefaultDockerComposeConfig = "./conf/docker/docker-compose.yaml"
	DefaultDockerComposeConfig = "./conf/docker/docker-compose.yaml"

	// CMComposeProjectName is a variable that holds the default COMPOSE_PROJECT_NAME that CM-Mayfly will use.
	CMComposeProjectName string = "cm-mayfly"
)

// SysCallDockerComposePs executes `docker-compose ps` command via system call.
func SysCallDockerComposePs() {
	fmt.Println("\n[v]Status of Cloud-Migrator runtimes")
	//cmdStr := "COMPOSE_PROJECT_NAME=cm-mayfly docker-compose -f " + DockerFilePath + " ps"
	cmdStr := fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s ps", CMComposeProjectName, DockerFilePath)
	common.SysCall(cmdStr)
}
