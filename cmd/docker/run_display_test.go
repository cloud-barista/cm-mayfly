package docker

import (
	"strings"
	"testing"
)

// showServiceInfo reads its services out of a map, whose iteration order differs
// on every run. Sorting each category by name is what keeps two runs of the same
// lineup from printing the services in two different orders.
func TestShowServiceInfoIsDeterministic(t *testing.T) {
	services := map[string]ServiceInfo{}
	for _, name := range []string{
		"cb-tumblebug", "cb-spider", "cb-mapui", "mc-terrarium",
		"cm-ant", "cm-beetle", "cm-honeybee",
		"openbao", "openbao-unseal", "ant-postgres", "cm-grasshopper-rustfs",
	} {
		services[name] = ServiceInfo{
			Name:     name,
			Image:    "example/" + name + ":1.0.0",
			Category: categorizeService(name, ""),
		}
	}

	first := captureStdout(t, func() { showServiceInfo(services) })
	for i := 0; i < 5; i++ {
		if again := captureStdout(t, func() { showServiceInfo(services) }); again != first {
			t.Fatalf("run %d printed a different listing:\n%s\n---\n%s", i+2, first, again)
		}
	}

	// Within a category the names are in ascending order.
	var core []string
	for _, line := range strings.Split(first, "\n") {
		for _, name := range []string{"cb-mapui", "cb-spider", "cb-tumblebug", "mc-terrarium"} {
			if strings.HasPrefix(line, "│ "+name+" ") {
				core = append(core, name)
			}
		}
	}
	want := "cb-mapui|cb-spider|cb-tumblebug|mc-terrarium"
	if got := strings.Join(core, "|"); got != want {
		t.Errorf("Core Infrastructure order = %q, want %q", got, want)
	}
}

// The two screens read one list, so they cannot drift into different orders or
// different icons for the same category.
func TestCategoryDisplayOrderCoversEveryCategory(t *testing.T) {
	listed := make(map[string]bool, len(categoryDisplayOrder))
	for _, entry := range categoryDisplayOrder {
		if listed[entry.Name] {
			t.Errorf("category %q is listed twice", entry.Name)
		}
		listed[entry.Name] = true
	}

	for _, category := range []string{
		CategoryCoreInfra, CategoryFrameworks, CategoryWebConsole, CategoryWorkflow,
		CategorySecrets, CategoryDataStores, CategoryObjectStorage, CategoryDependencies,
	} {
		if !listed[category] {
			t.Errorf("category %q has no display order entry", category)
		}
		if categoryIcon(category) == "" {
			t.Errorf("category %q has no icon", category)
		}
	}
}
