package common

// API Call
const (
	API_FILE  = "./conf/api.yaml"
	SWAG_FILE = "./conf/swagger.json"
)

// Docker
const (
	DefaultDockerComposeConfig = "./conf/docker/docker-compose.yaml"

	// CMComposeProjectName is a variable that holds the default COMPOSE_PROJECT_NAME that cloud-migrator will use.
	ComposeProjectName string = "cloud-migrator"
)

// k8s
const (
	DefaultKubernetesConfig string = "./conf/k8s/cloud-migrator/values.yaml"

	// CMK8sNamespace is a variable that holds the K8s namespace that CM-Mayfly will use.
	CMK8sNamespace string = "cloud-migrator"

	// CMHelmReleaseName is a variable that holds the K8s Helm release name that CM-Mayfly will use.
	CMHelmReleaseName string = "cloud-migrator"
)
