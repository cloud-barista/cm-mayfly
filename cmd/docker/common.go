package docker

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/cm-mayfly/cm-mayfly/common"
	"gopkg.in/yaml.v3"
)

// ServiceInfo represents service information with category
type ServiceInfo struct {
	Name     string
	Image    string
	Category string
}

// ComposeService is one entry under the compose file's top-level `services:` key.
//
// Repository and Tag are the two halves of Image; they are split once here so
// callers never have to re-parse the string (and never disagree about where the
// split is for images that carry a registry host or port).
type ComposeService struct {
	Name       string
	Image      string
	Repository string
	Tag        string
	Category   string
	DependsOn  []string
}

// ComposeFile is the parsed compose file. Order preserves the order the
// services are written in, which is what the info tables list them in.
type ComposeFile struct {
	Order    []string
	Services map[string]ComposeService
}

// composeCache memoises the parse for the lifetime of the process. cm-mayfly is
// a one-shot CLI, but `infra info` asks for the same file once per service, so
// without this the whole file is re-read and re-parsed ~20 times per run.
var (
	composeCacheMu  sync.Mutex
	composeCacheKey string
	composeCacheSet bool
	composeCacheFmt *ComposeFile
	composeCacheErr error
)

// composeCacheKeyFor identifies a parse by path, size and modification time, so
// a file edited between two calls is re-read rather than served from the cache.
func composeCacheKeyFor(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return path + "|missing"
	}
	return fmt.Sprintf("%s|%d|%d", path, info.Size(), info.ModTime().UnixNano())
}

// loadComposeFile parses DockerFilePath and returns its services.
func loadComposeFile() (*ComposeFile, error) {
	composeCacheMu.Lock()
	defer composeCacheMu.Unlock()

	key := composeCacheKeyFor(DockerFilePath)
	if composeCacheSet && composeCacheKey == key {
		return composeCacheFmt, composeCacheErr
	}

	var parsed *ComposeFile
	content, err := os.ReadFile(DockerFilePath)
	if err != nil {
		err = fmt.Errorf("failed to read %s: %v", DockerFilePath, err)
	} else {
		parsed, err = parseComposeContent(content)
	}

	composeCacheKey = key
	composeCacheSet = true
	composeCacheFmt = parsed
	composeCacheErr = err
	return parsed, err
}

// parseComposeContent parses compose YAML into a ComposeFile.
//
// Only the top-level keys under `services:` count as services. The previous
// line-oriented scanner keyed off "a bare `key:` line followed by an `image:`
// line", which silently dropped any service that declared `build:` or
// `healthcheck:` before `image:` — and a dropped service is reported to the
// user as "not found in docker-compose.yaml".
func parseComposeContent(content []byte) (*ComposeFile, error) {
	var root struct {
		Services yaml.Node `yaml:"services"`
	}
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("failed to parse compose YAML: %v", err)
	}

	result := &ComposeFile{Services: make(map[string]ComposeService)}
	if root.Services.Kind != yaml.MappingNode {
		return result, nil
	}

	// A mapping node stores keys and values as alternating children.
	for i := 0; i+1 < len(root.Services.Content); i += 2 {
		name := root.Services.Content[i].Value
		if name == "" {
			continue
		}

		var body struct {
			Image     string    `yaml:"image"`
			DependsOn yaml.Node `yaml:"depends_on"`
		}
		if err := root.Services.Content[i+1].Decode(&body); err != nil {
			// A service we cannot decode is still a service: record the name so
			// it validates, rather than telling the user it does not exist.
			result.Order = append(result.Order, name)
			result.Services[name] = ComposeService{Name: name, Category: categorizeService(name, "")}
			continue
		}

		repository, tag := splitImageRef(body.Image)
		svc := ComposeService{
			Name:       name,
			Image:      body.Image,
			Repository: repository,
			Tag:        tag,
			Category:   categorizeService(name, repository),
			DependsOn:  decodeDependsOn(&body.DependsOn),
		}

		result.Order = append(result.Order, name)
		result.Services[name] = svc
	}

	return result, nil
}

// splitImageRef splits an image reference into repository and tag.
//
// The split is on the last colon, but only when that colon belongs to a tag
// rather than to a registry port: "localhost:5000/foo" has no tag, while
// "localhost:5000/foo:1.2" does. Digest references ("repo@sha256:…") have no
// tag either.
func splitImageRef(image string) (string, string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return "", ""
	}
	if at := strings.Index(image, "@"); at >= 0 {
		return image[:at], ""
	}

	colon := strings.LastIndex(image, ":")
	if colon < 0 {
		return image, ""
	}
	if strings.Contains(image[colon+1:], "/") {
		// The colon was a registry port, not a tag separator.
		return image, ""
	}
	return image[:colon], image[colon+1:]
}

// decodeDependsOn reads compose's two depends_on spellings: the short list form
// ("- cb-spider") and the long map form ("cb-spider: {condition: …}").
func decodeDependsOn(node *yaml.Node) []string {
	if node == nil || node.Kind == 0 {
		return nil
	}

	switch node.Kind {
	case yaml.SequenceNode:
		var names []string
		for _, item := range node.Content {
			if item.Value != "" {
				names = append(names, item.Value)
			}
		}
		return names
	case yaml.MappingNode:
		var names []string
		for i := 0; i+1 < len(node.Content); i += 2 {
			if name := node.Content[i].Value; name != "" {
				names = append(names, name)
			}
		}
		return names
	}
	return nil
}

// parseDockerComposeImages parses docker-compose.yaml to extract all service information
func parseDockerComposeImages() (map[string]ServiceInfo, error) {
	parsed, err := loadComposeFile()
	if err != nil {
		return nil, err
	}

	services := make(map[string]ServiceInfo, len(parsed.Services))
	for name, svc := range parsed.Services {
		services[name] = ServiceInfo{
			Name:     svc.Name,
			Image:    svc.Image,
			Category: svc.Category,
		}
	}
	return services, nil
}

// splitServiceNames normalizes a raw -s value into a list of service names.
//
// Commas and whitespace are both accepted, and may be mixed freely
// ("a, b c" → ["a" "b" "c"]). Empty fields are dropped and duplicates are
// removed while preserving the order the user wrote them in.
//
// A raw value that holds nothing but separators (",", "  ") yields an empty
// slice. Callers must NOT read that as "all services" — see resolveServices.
func splitServiceNames(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})

	seen := make(map[string]bool, len(fields))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		name := strings.TrimSpace(f)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// resolveServices splits a -s value and validates each name against the compose
// file. It is the single entry point every infra subcommand uses to work out
// which services it was asked to act on.
//
// The return value distinguishes two cases that must never be confused:
//
//   - raw is empty (the -s flag was not given) → (nil, nil), meaning "all
//     services". This is the only way to select the whole environment.
//   - raw is non-empty → a non-empty, validated list, or an error.
//
// In particular a raw value that contains only separators (-s "," or -s " ")
// is an error, not "all services". Treating it as the whole environment is how
// a typo aimed at one service ends up tearing down everything — which is why
// the empty check below is on raw itself rather than on a trimmed copy.
func resolveServices(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}

	names := splitServiceNames(raw)
	if len(names) == 0 {
		return nil, fmt.Errorf("no service name found in -s %q\n"+
			"The value holds only separators. Name the services you want, e.g. -s cb-spider or -s \"cb-spider cb-tumblebug\".\n"+
			"Omit -s entirely to target every service", raw)
	}

	available, err := parseDockerComposeImages()
	if err != nil {
		return nil, err
	}

	return validateServiceNames(names, available)
}

// resolveSelectedServices turns the -s flag values into a validated service
// list. The flag is repeatable and each occurrence may itself hold several
// names separated by commas or spaces, so the occurrences are joined and handed
// to resolveServices — the single splitting/validating path every subcommand
// shares. All three forms therefore select the same services:
//
//	-s cb-spider -s cb-tumblebug
//	-s cb-spider,cb-tumblebug
//	-s "cb-spider cb-tumblebug"
//
// Omitting -s entirely still means "every service", and a value that holds only
// separators is still an error rather than "all" — see resolveServices.
func resolveSelectedServices() ([]string, error) {
	return resolveServices(strings.Join(ServiceNames, ","))
}

// validateServiceNames checks each already-split name against the services
// declared in the compose file. It is split out from resolveServices so the
// validation can be exercised without touching the filesystem.
func validateServiceNames(names []string, available map[string]ServiceInfo) ([]string, error) {
	var unknown []string
	for _, name := range names {
		if _, exists := available[name]; !exists {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) == 0 {
		return names, nil
	}

	// Name the offending entries only. Reporting the whole -s string leaves the
	// user to work out which of several names was wrong.
	subject := fmt.Sprintf("Service '%s' was", unknown[0])
	if len(unknown) > 1 {
		subject = fmt.Sprintf("Services '%s' were", strings.Join(unknown, "', '"))
	}

	sorted := make([]string, 0, len(available))
	for name := range available {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)

	var b strings.Builder
	fmt.Fprintf(&b, "%s not found in %s\nAvailable services:\n", subject, DockerFilePath)
	for _, name := range sorted {
		fmt.Fprintf(&b, "  - %s\n", name)
	}
	return nil, errors.New(strings.TrimRight(b.String(), "\n"))
}

// composeArgs builds the argument vector for a `docker compose -f <file> …`
// invocation, with the caller's arguments appended.
func composeArgs(args ...string) []string {
	return append([]string{"compose", "-f", DockerFilePath}, args...)
}

// composeEnv returns the environment additions every compose invocation needs.
// COMPOSE_PROJECT_NAME is passed through the child environment rather than
// prefixed onto a shell command string.
func composeEnv() []string {
	return []string{"COMPOSE_PROJECT_NAME=" + ProjectName}
}

// runCompose runs `docker compose -f <file> <args…>` without a shell and
// returns the exit status.
func runCompose(args ...string) error {
	return common.RunCommand("docker", composeArgs(args...), composeEnv())
}

// composeOutput runs `docker compose -f <file> <args…>` without a shell and
// returns its standard output.
func composeOutput(args ...string) ([]byte, error) {
	return common.RunCommandOutput("docker", composeArgs(args...), composeEnv())
}

// displayCommand renders an argument vector for display (dry-run output, error
// messages). It is never handed to a shell.
func displayCommand(name string, args []string) string {
	return name + " " + strings.Join(args, " ")
}

// Service categories. A service belongs to the framework it is part of first,
// and to its technical role second: cb-tumblebug-postgres is a data store of the
// Core Infrastructure lineup, while cm-butterfly-db belongs with the console it
// serves. Grouping on the image prefix alone cannot express that, because one
// registry namespace publishes images for every framework.
const (
	CategoryCoreInfra     = "Core Infrastructure"
	CategoryFrameworks    = "C-Mig Frameworks"
	CategoryWebConsole    = "Web Console"
	CategoryWorkflow      = "Workflow Engine"
	CategorySecrets       = "Secrets"
	CategoryDataStores    = "Data Stores"
	CategoryObjectStorage = "Object Storage"
	CategoryDependencies  = "Dependencies"
)

// serviceCategories maps every service the shipped compose file declares to its
// category. This is the authority: the pattern rules below only cover services
// that are not listed here.
var serviceCategories = map[string]string{
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

// CategoryDisplay pairs a category with the icon that heads it on screen.
type CategoryDisplay struct {
	Name string
	Icon string
}

// categoryDisplayOrder is the order categories are shown in, and the icon each
// one carries. Both `infra run` and `infra info --human` read it, so the two
// screens cannot drift into different orders or different icons.
var categoryDisplayOrder = []CategoryDisplay{
	{CategoryCoreInfra, "🎯"},
	{CategoryFrameworks, "🧩"},
	{CategoryWebConsole, "🖥️"},
	{CategoryWorkflow, "⚙️"},
	{CategorySecrets, "🔐"},
	{CategoryDataStores, "🗄️"},
	{CategoryObjectStorage, "📦"},
	{CategoryDependencies, "🔧"},
}

// unknownCategoryIcon heads a category that categoryDisplayOrder does not know
// about, which can only happen if a new category is added without listing it.
const unknownCategoryIcon = "🔧"

// categoryIcon returns the icon a category is headed with.
func categoryIcon(name string) string {
	for _, entry := range categoryDisplayOrder {
		if entry.Name == name {
			return entry.Icon
		}
	}
	return unknownCategoryIcon
}

// categorizeService returns the category a service is displayed under. Names
// listed in serviceCategories always win; anything else falls through to the
// pattern rules so a service added to the compose file still lands somewhere
// sensible instead of disappearing.
func categorizeService(serviceName, imageName string) string {
	if category, ok := serviceCategories[serviceName]; ok {
		return category
	}

	switch {
	case strings.HasPrefix(serviceName, "airflow-"):
		return CategoryWorkflow
	case strings.HasPrefix(serviceName, "openbao"):
		return CategorySecrets
	case strings.HasPrefix(serviceName, "cm-butterfly-"):
		return CategoryWebConsole
	case strings.HasSuffix(serviceName, "-postgres"), strings.HasSuffix(serviceName, "-mysql"),
		strings.HasSuffix(serviceName, "-db"), strings.Contains(serviceName, "etcd"),
		strings.Contains(serviceName, "redis"):
		return CategoryDataStores
	case strings.HasPrefix(serviceName, "cb-"), serviceName == "mc-terrarium":
		return CategoryCoreInfra
	case strings.HasPrefix(serviceName, "cm-"):
		return CategoryFrameworks
	}

	return CategoryDependencies
}
