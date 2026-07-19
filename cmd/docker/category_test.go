package docker

import (
	"path/filepath"
	"testing"
)

// The category every service in the shipped compose file must land in. The
// grouping follows the framework a service belongs to before its technical
// role, which is why cb-tumblebug-postgres is a data store of the core lineup
// while cm-butterfly-db sits with the console it serves.
var expectedComposeCategories = map[string]string{
	"cb-spider":    CategoryCoreInfra,
	"cb-tumblebug": CategoryCoreInfra,
	"cb-mapui":     CategoryCoreInfra,
	"mc-terrarium": CategoryCoreInfra,

	"cm-beetle":      CategoryFrameworks,
	"cm-honeybee":    CategoryFrameworks,
	"cm-damselfly":   CategoryFrameworks,
	"cm-grasshopper": CategoryFrameworks,
	"cm-ant":         CategoryFrameworks,

	"cm-butterfly-front": CategoryWebConsole,
	"cm-butterfly-api":   CategoryWebConsole,
	"cm-butterfly-db":    CategoryWebConsole,

	"cm-cicada":      CategoryWorkflow,
	"airflow-server": CategoryWorkflow,
	"airflow-mysql":  CategoryWorkflow,
	"airflow-redis":  CategoryWorkflow,

	"openbao":        CategorySecrets,
	"openbao-unseal": CategorySecrets,

	"cb-tumblebug-etcd":     CategoryDataStores,
	"cb-tumblebug-postgres": CategoryDataStores,
	"ant-postgres":          CategoryDataStores,

	"cm-grasshopper-rustfs": CategoryObjectStorage,
}

// withShippedCompose points DockerFilePath at conf/docker/docker-compose.yaml so
// a test reads the lineup that actually ships.
func withShippedCompose(t *testing.T) {
	t.Helper()

	prev := DockerFilePath
	DockerFilePath = filepath.Join("..", "..", "conf", "docker", "docker-compose.yaml")
	t.Cleanup(func() { DockerFilePath = prev })
}

// Every service in the shipped compose file lands in its expected group. The
// image prefix used to be checked first, so airflow-server was filed under the
// core services on the strength of its cloudbaristaorg/ image and never reached
// the airflow rule, while airflow-mysql and airflow-redis scattered into the
// database and cache groups. The workflow engine was split across three groups.
func TestCategorizeServiceMatchesShippedCompose(t *testing.T) {
	withShippedCompose(t)

	compose, err := loadComposeFile()
	if err != nil {
		t.Fatalf("failed to parse the shipped compose file: %v", err)
	}

	for name, want := range expectedComposeCategories {
		svc, ok := compose.Services[name]
		if !ok {
			t.Errorf("%s is missing from the shipped compose file", name)
			continue
		}
		if svc.Category != want {
			t.Errorf("%s: category = %q, want %q (image %q)", name, svc.Category, want, svc.Image)
		}
	}
}

// No service may go missing from the grouped view and none may be counted
// twice: the sum over the categories has to equal the number of services the
// compose file declares.
func TestEveryComposeServiceHasExactlyOneCategory(t *testing.T) {
	withShippedCompose(t)

	compose, err := loadComposeFile()
	if err != nil {
		t.Fatalf("failed to parse the shipped compose file: %v", err)
	}

	if len(compose.Services) != len(expectedComposeCategories) {
		t.Errorf("compose declares %d services but the expected table lists %d; update the table",
			len(compose.Services), len(expectedComposeCategories))
	}

	counts := make(map[string]int)
	for name, svc := range compose.Services {
		if svc.Category == "" {
			t.Errorf("%s has no category", name)
			continue
		}
		if _, known := expectedComposeCategories[name]; !known {
			t.Errorf("%s is in the compose file but not in the expected table (category %q)", name, svc.Category)
		}
		counts[svc.Category]++
	}

	total := 0
	for _, n := range counts {
		total += n
	}
	if total != len(compose.Services) {
		t.Errorf("categories hold %d services, compose declares %d", total, len(compose.Services))
	}

	// The fallback group exists for services the mapping does not know, and the
	// shipped lineup is fully mapped, so it must be empty.
	if n := counts[CategoryDependencies]; n != 0 {
		t.Errorf("%d service(s) fell through to %s", n, CategoryDependencies)
	}
}

// A service the mapping does not list still has to land somewhere, and the
// pattern rules must not override an explicit mapping.
func TestCategorizeServiceFallbacks(t *testing.T) {
	cases := []struct {
		service string
		image   string
		want    string
	}{
		{"airflow-worker", "cloudbaristaorg/airflow-worker:1.0.0", CategoryWorkflow},
		{"openbao-agent", "openbao/openbao:2.5.1", CategorySecrets},
		{"cm-butterfly-cache", "redis:7.2-alpine", CategoryWebConsole},
		{"cm-beetle-postgres", "postgres:16-alpine", CategoryDataStores},
		{"some-mysql", "mysql:8.0-debian", CategoryDataStores},
		{"cache-redis", "redis:7.2-alpine", CategoryDataStores},
		{"cluster-etcd", "gcr.io/etcd-development/etcd:v3.6.11", CategoryDataStores},
		{"cb-larva", "cloudbaristaorg/cb-larva:1.0.0", CategoryCoreInfra},
		{"mc-terrarium", "cloudbaristaorg/mc-terrarium:0.1.4", CategoryCoreInfra},
		{"cm-dragonfly", "cloudbaristaorg/cm-dragonfly:1.0.0", CategoryFrameworks},
		{"nginx", "nginx:1.27", CategoryDependencies},
		{"", "", CategoryDependencies},
	}

	for _, tc := range cases {
		if got := categorizeService(tc.service, tc.image); got != tc.want {
			t.Errorf("categorizeService(%q, %q) = %q, want %q", tc.service, tc.image, got, tc.want)
		}
	}
}

// A cloudbaristaorg/ image no longer decides the group on its own. This is the
// regression that filed airflow-server under the core services.
func TestCategorizeServiceIgnoresImagePrefix(t *testing.T) {
	if got := categorizeService("airflow-server", "cloudbaristaorg/airflow-server"); got != CategoryWorkflow {
		t.Errorf("airflow-server = %q, want %q", got, CategoryWorkflow)
	}
	if got := categorizeService("cm-butterfly-db", "postgres"); got != CategoryWebConsole {
		t.Errorf("cm-butterfly-db = %q, want %q", got, CategoryWebConsole)
	}
}
