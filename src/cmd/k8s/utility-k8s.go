package k8s

var (
	// K8sFilePath is a variable that holds path to the helm-chart's values.yaml.
	K8sFilePath string

	serviceNameList = []string{"cb-spider"}
)

const (
	// DefaultKubernetesConfig is a variable that holds path to cloud-migrator/values.yaml
	DefaultKubernetesConfig string = "../conf/k8s/cloud-migrator/values.yaml"

	// CMK8sNamespace is a variable that holds the K8s namespace that CM-Mayfly will use.
	CMK8sNamespace string = "cloud-migrator"

	// CMHelmReleaseName is a variable that holds the K8s Helm release name that CM-Mayfly will use.
	CMHelmReleaseName string = "cloud-migrator"
)
