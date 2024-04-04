package k8s

// K8sFilePath is a variable that holds path to the helm-chart's values.yaml.
var K8sFilePath string

const (
	// DefaultKubernetesConfig is a variable that holds path to helm-chart/values.yaml
	DefaultKubernetesConfig string = "../helm-chart/values.yaml"

	// CMK8sNamespace is a variable that holds the K8s namespace that CM-Mayfly will use.
	CMK8sNamespace string = "cm-mayfly"

	// CMHelmReleaseName is a variable that holds the K8s Helm release name that CM-Mayfly will use.
	CMHelmReleaseName string = "cm-mayfly"
)
