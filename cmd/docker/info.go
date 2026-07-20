package docker

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/cm-mayfly/cm-mayfly/internal/openbao"
	"github.com/spf13/cobra"
)

// printOpenbaoInfo appends a compact OpenBao consistency section to `infra info`.
// It reuses the shared preflight (read-only) so the same verdict shown by
// `setup openbao status` is surfaced here.
func printOpenbaoInfo() {
	fmt.Println()
	fmt.Println("[OpenBao consistency]")
	fmt.Println(openbao.CompactStatus())
}

// openbaoRelevant reports whether the OpenBao consistency section is worth
// printing for this invocation.
//
// With no -s the report covers the whole environment, so it always is. With a
// -s filter it only is when OpenBao itself is among the services being shown —
// otherwise `infra info -s cm-ant` ends with a verdict about a component the
// user did not ask about.
func openbaoRelevant(services []string) bool {
	if len(services) == 0 {
		return true
	}
	for _, name := range services {
		if strings.HasPrefix(name, "openbao") {
			return true
		}
	}
	return false
}

// infoAllFlag represents the --all flag for showing all containers including stopped ones
var infoAllFlag bool

// infoHumanFlag represents the --human flag for showing human-readable service status
var infoHumanFlag bool

// infoTestVersionsFlag represents the --test-versions flag for debugging version extraction
var infoTestVersionsFlag bool

// infoCmd represents the info command.
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get information of Cloud-Migrator System",
	Long:  `Get information of Cloud-Migrator System. Information about containers and container images`,
	Run: func(cmd *cobra.Command, args []string) {
		if infoTestVersionsFlag {
			showVersionTestInfo()
		} else if infoHumanFlag {
			showHumanReadableInfo()
		} else {
			fmt.Println("\n[Get info for Cloud-Migrator runtimes]")
			fmt.Println()
			SysCallDockerComposePsWithAll(infoAllFlag)
			fmt.Println("")

			fmt.Println("[v]Status of Cloud-Migrator runtime images")
			services, err := resolveServices(ServiceName)
			if err != nil {
				fmt.Printf("❌ %v\n", err)
				return
			}
			if err := runCompose(append([]string{"images"}, services...)...); err != nil {
				fmt.Printf("❌ docker compose images failed: %v\n", err)
			}

			// OpenBao consistency summary (shared preflight, read-only).
			if openbaoRelevant(services) {
				printOpenbaoInfo()
			}

			// Add helpful hint about --human option
			fmt.Println()
			fmt.Println("💡 Tip: Use '--human' or '-u' option for a more readable service status table:")
			fmt.Println("   ./mayfly infra info --human")
		}
	},
}

func init() {
	dockerCmd.AddCommand(infoCmd)

	// Add --all flag to info command. It only affects the default view: the
	// --human and --test-versions tables list every service in the compose file
	// by design, so they have nothing to widen.
	infoCmd.Flags().BoolVarP(&infoAllFlag, "all", "a", false, "Show all containers including stopped ones (default view only; --human and --test-versions always list every service)")
	// Add --human flag to info command
	infoCmd.Flags().BoolVarP(&infoHumanFlag, "human", "u", false, "Show human-readable service status table")
	// Add --test-versions flag to info command
	infoCmd.Flags().BoolVarP(&infoTestVersionsFlag, "test-versions", "t", false, "Test version extraction from docker-compose.yaml and show service status")

	// --human and --test-versions each render their own table; asking for both
	// used to run --test-versions and drop the other request without a word.
	infoCmd.MarkFlagsMutuallyExclusive("human", "test-versions")
}

// Glyphs the VERSION column of the --human table uses.
//
// The column always shows the tag docker-compose.yaml names for the service —
// the version that is supposed to come up — followed by one mark telling
// whether that is what is actually running. Spelling the state out in
// parentheses ("(?)", "(Not Downloaded)") doubled the column width and still
// left the reader unable to see which version was really up.
const (
	// versionRunning marks a compose version that is the one actually running.
	versionRunning = "✓"
	// versionNotRunning marks a compose version that is not running, whether
	// because the service is down or because its image is not even local.
	versionNotRunning = "✗"
	// actualVersionPrefix introduces the follow-up row naming the version that
	// is running instead. Without it two bare versions on consecutive rows read
	// as two services.
	actualVersionPrefix = "<- "
)

// HumanServiceInfo represents service information for human-readable display
type HumanServiceInfo struct {
	Service      string
	Version      string
	Status       string
	Healthy      string
	InternalPort string
	ExternalPort string
	ImageSize    string

	// ActualVersion is set only when the service is running on a version other
	// than the one compose names. It renders as an extra row under the service,
	// and the block gets ruled off so the disagreement is hard to miss.
	ActualVersion string
	// ActualHealthy is the health of that actually-running version.
	ActualHealthy string
}

// showHumanReadableInfo displays service information in a human-readable table format
func showHumanReadableInfo() {
	fmt.Println("\n[Cloud-Migrator Service Status]")
	fmt.Println()

	// Get services from docker-compose.yaml
	allServices := getServicesFromCompose()

	// Filter services if -s option is used
	requested, err := resolveServices(ServiceName)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return
	}

	var services []string
	if len(requested) > 0 {
		services = append(services, requested...)

		// Add dependency services
		dependencyServices := getDependencyServices(services)
		services = append(services, dependencyServices...)

		// Remove duplicates
		services = removeDuplicateServices(services)
	} else {
		services = allServices
	}

	// Get container information - always show all containers for human-readable format
	containers := getContainerInfo(true)

	// Get image information
	images := getImageInfo()
	localSizes := getLocalImageSizes()

	// Create service info table
	var serviceInfos []HumanServiceInfo

	for _, service := range services {
		info := HumanServiceInfo{
			Service: service,
		}

		container, running := containers[service]

		// Get container info for this service
		if running {
			info.Status = container.Status
			info.Healthy = container.Healthy
			info.InternalPort = container.InternalPort
			info.ExternalPort = container.ExternalPort
		} else {
			info.Status = "Not Found"
			info.Healthy = "-"
			info.InternalPort = "-"
			info.ExternalPort = "-"
		}

		// The VERSION column is the compose tag either way; what changes is
		// whether it is marked as the one actually running.
		info.Version, info.ActualVersion = humanVersionCells(service, container, running)
		if info.ActualVersion != "" {
			info.ActualHealthy = container.Healthy
		}

		// Get image size - prioritize running container's image
		if running && container.Status == "running" {
			// A running container may be on an image other than the one the
			// compose file names, so size is looked up by the image it actually
			// runs rather than by the expected tag.
			if size, exists := localSizes[container.Image]; exists {
				info.ImageSize = size
			} else if image, exists := images[service]; exists {
				info.ImageSize = image.Size
			} else {
				info.ImageSize = "Running"
			}
		} else {
			// For non-running services, check if expected version image exists.
			// A missing image needs no words here: the "✗" beside the version
			// already says the service is not up on it.
			if image, exists := images[service]; exists {
				info.ImageSize = image.Size
			} else {
				info.ImageSize = "-"
			}
		}

		serviceInfos = append(serviceInfos, info)
	}

	// Display table with service categorization
	displayServiceTableWithDependencies(serviceInfos, requested)

	// OpenBao consistency summary (shared preflight, read-only).
	if openbaoRelevant(services) {
		printOpenbaoInfo()
	}
}

// ContainerInfo represents container information
type ContainerInfo struct {
	Status       string
	Healthy      string
	InternalPort string
	ExternalPort string
	Version      string
	Image        string
}

// ImageInfo represents image information
type ImageInfo struct {
	Size string
	Tag  string
}

// getServicesFromCompose extracts service names from docker-compose.yaml, in
// the order they are declared.
//
// This used to be a hardcoded literal, which meant the compose file and this
// list were two independent sources for "what services exist". They drifted:
// cm-grasshopper-rustfs was added to compose but not here, so `run -s` accepted
// the name while `info --human` said it did not exist.
func getServicesFromCompose() []string {
	parsed, err := loadComposeFile()
	if err != nil {
		return nil
	}
	return append([]string(nil), parsed.Order...)
}

// getContainerInfo gets container information using docker compose ps
func getContainerInfo(showAll bool) map[string]ContainerInfo {
	containers := make(map[string]ContainerInfo)

	// Execute docker compose ps command
	args := []string{"ps"}
	if showAll {
		args = append(args, "-a")
	}
	args = append(args, "--format", "json")

	output, err := composeOutput(args...)
	if err != nil {
		return containers
	}

	// Parse JSON output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var container struct {
			Name    string `json:"Name"`
			Service string `json:"Service"`
			State   string `json:"State"`
			Status  string `json:"Status"`
			Health  string `json:"Health"`
			Ports   string `json:"Ports"`
			Image   string `json:"Image"`
		}

		if err := json.Unmarshal([]byte(line), &container); err != nil {
			continue
		}

		// Parse ports
		internalPort, externalPort := parsePorts(container.Ports)

		// Determine health status
		healthy := "-"
		if container.Health != "" {
			// Use Health field if available (more accurate)
			if container.Health == "healthy" {
				healthy = "✓"
			} else if container.Health == "unhealthy" {
				healthy = "✗"
			} else if container.Health == "starting" {
				healthy = "starting"
			}
		} else {
			// Fallback to Status field parsing
			if strings.Contains(container.Status, "healthy") {
				healthy = "✓"
			} else if strings.Contains(container.Status, "unhealthy") {
				healthy = "✗"
			}
		}

		// Extract version from image tag if available, otherwise use fallback
		version := extractVersionFromImage(container.Image)
		if version == "" {
			// If running container has no version tag (e.g., image ID), get from docker-compose.yaml
			version = getVersionFromComposeFile(container.Service)
		}

		// Normalize status display
		status := container.State
		if container.State == "exited" {
			status = "Stopped"
		}

		containers[container.Service] = ContainerInfo{
			Status:       status,
			Healthy:      healthy,
			InternalPort: internalPort,
			ExternalPort: externalPort,
			Version:      version,
			Image:        container.Image,
		}
	}

	return containers
}

// extractVersionFromImage extracts version/tag from image name
func extractVersionFromImage(imageName string) string {
	if imageName == "" {
		return ""
	}

	// Check if imageName is a SHA256 image ID (sha256:64-character-hex)
	if strings.HasPrefix(imageName, "sha256:") {
		hash := strings.TrimPrefix(imageName, "sha256:")
		if len(hash) == 64 && isHexString(hash) {
			return "" // Return empty to trigger fallback to getVersionFromComposeFile
		}
	}

	// Check if imageName is a short image ID (12 character hex string)
	if len(imageName) == 12 && isHexString(imageName) {
		return "" // Return empty to trigger fallback to getVersionFromComposeFile
	}

	// Split by colon to get tag part
	parts := strings.Split(imageName, ":")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}

	return ""
}

// isHexString checks if a string contains only hexadecimal characters
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// localImageSizes caches `docker images` for the lifetime of the process. The
// info tables ask about every service, and each lookup used to spawn its own
// `docker images` — about twenty processes for one `info --human`.
var (
	localImageOnce  sync.Once
	localImageIndex map[string]string
)

// getLocalImageSizes returns a "repository:tag" → size index of the images
// present locally.
func getLocalImageSizes() map[string]string {
	localImageOnce.Do(func() {
		localImageIndex = make(map[string]string)

		output, err := common.RunCommandOutput("docker", []string{"images", "--format", "json"}, nil)
		if err != nil {
			return
		}

		for _, line := range strings.Split(string(output), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}

			var image struct {
				Repository string `json:"Repository"`
				Tag        string `json:"Tag"`
				Size       string `json:"Size"`
			}
			if err := json.Unmarshal([]byte(line), &image); err != nil {
				continue
			}
			if image.Repository == "" || image.Repository == "<none>" || image.Tag == "<none>" {
				continue
			}

			localImageIndex[image.Repository+":"+image.Tag] = image.Size
		}
	})

	return localImageIndex
}

// getImageInfo reports, per service, the local image matching the reference the
// compose file names for it.
//
// The match is on the exact "repository:tag" pair. Matching loosely — the old
// code took any repository *containing* a keyword and fell back to the first
// hit when the expected tag was absent — produced rows that contradicted
// themselves: cb-tumblebug-postgres read "16-alpine (Not Downloaded)" and
// "291MB" at once, the 291MB being the unrelated postgres:14-alpine that
// cm-butterfly-db uses. A service whose exact image is not local now has no
// entry, and the caller renders that as "Not Downloaded".
func getImageInfo() map[string]ImageInfo {
	images := make(map[string]ImageInfo)

	parsed, err := loadComposeFile()
	if err != nil {
		return images
	}

	local := getLocalImageSizes()
	for name, svc := range parsed.Services {
		if svc.Repository == "" || svc.Tag == "" {
			continue
		}
		if size, exists := local[svc.Repository+":"+svc.Tag]; exists {
			images[name] = ImageInfo{Size: size, Tag: svc.Tag}
		}
	}

	return images
}

// composeVersion returns the tag docker-compose.yaml names for a service and
// whether that exact image is present locally.
//
// The two facts are kept apart so that callers append at most one qualifier to
// the version string. When they were baked into one return value the human
// table produced "0.12.9 (Not Downloaded) (?)" — two qualifiers stacked, one
// from here and one from the caller, and a version column padded to fit both.
func composeVersion(serviceName string) (version string, present bool) {
	parsed, err := loadComposeFile()
	if err != nil {
		return "", false
	}

	svc, exists := parsed.Services[serviceName]
	if !exists || svc.Tag == "" {
		return "", false
	}

	_, present = getLocalImageSizes()[svc.Repository+":"+svc.Tag]
	return svc.Tag, present
}

// getVersionFromComposeFile reads docker-compose.yaml and returns version for non-running services
func getVersionFromComposeFile(serviceName string) string {
	version, present := composeVersion(serviceName)
	if version == "" {
		return "-"
	}
	if !present {
		return version + " (Not Downloaded)"
	}
	return version
}

// humanVersionCells renders the VERSION cell of the --human table for a service,
// plus the version of a differing image that is actually running, if any.
//
// The first return value is always the compose tag with one mark appended: "✓"
// when that tag is what is up, "✗" otherwise. The second is empty unless the
// service runs on some other tag, in which case the caller adds a follow-up row
// naming it.
//
// Only a container in the "running" state can confirm a version. A stopped one
// says nothing about what would come up, so its image tag is not reported as
// actually running.
func humanVersionCells(serviceName string, container ContainerInfo, exists bool) (version, actual string) {
	composeTag, _ := composeVersion(serviceName)
	if composeTag == "" {
		return "-", ""
	}

	if !exists || container.Status != "running" {
		return composeTag + " " + versionNotRunning, ""
	}

	if container.Version == "" || container.Version == composeTag {
		return composeTag + " " + versionRunning, ""
	}

	return composeTag + " " + versionNotRunning, container.Version
}

// showVersionTestInfo displays version extraction test results and service status
func showVersionTestInfo() {
	fmt.Println("\n=== Version Extraction Test & Service Status ===")
	fmt.Println()

	// Honour -s, like the --human table does. Without this the flag was read,
	// validated, and then ignored: `info -t -s cm-ant` printed every service.
	requested, err := resolveServices(ServiceName)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return
	}

	services := requested
	if len(services) == 0 {
		services = getServicesFromCompose()
	}

	// Get running containers
	containers := getContainerInfo(false)

	// Get image information
	images := getImageInfo()

	type testRow struct {
		service        string
		composeVersion string
		status         string
		actualVersion  string
		imageSize      string
	}

	rows := make([]testRow, 0, len(services))
	for _, service := range services {
		row := testRow{
			service:        service,
			composeVersion: getVersionFromComposeFile(service),
			status:         "Not Running",
			actualVersion:  "-",
			imageSize:      "-",
		}

		if container, exists := containers[service]; exists {
			row.status = container.Status
			row.actualVersion = container.Version

			// Get image size
			if image, exists := images[service]; exists {
				row.imageSize = image.Size
			}
		}

		rows = append(rows, row)
	}

	// Size the columns from the content. The widths used to be fixed at 20 and
	// 15, which is narrower than real values such as "cb-tumblebug-postgres"
	// (21) and "16-alpine (Not Downloaded)" (26), so every long row pushed the
	// remaining columns out of alignment.
	serviceWidth := getDisplayWidth("SERVICE")
	composeWidth := getDisplayWidth("COMPOSE_VERSION")
	statusWidth := getDisplayWidth("STATUS")
	actualWidth := getDisplayWidth("ACTUAL_VERSION")
	for _, row := range rows {
		serviceWidth = maxInt(serviceWidth, getDisplayWidth(row.service))
		composeWidth = maxInt(composeWidth, getDisplayWidth(row.composeVersion))
		statusWidth = maxInt(statusWidth, getDisplayWidth(row.status))
		actualWidth = maxInt(actualWidth, getDisplayWidth(row.actualVersion))
	}

	rowFormat := fmt.Sprintf("%%-%ds %%-%ds %%-%ds %%-%ds %%s\n",
		serviceWidth, composeWidth, statusWidth, actualWidth)

	fmt.Printf(rowFormat, "SERVICE", "COMPOSE_VERSION", "STATUS", "ACTUAL_VERSION", "IMAGE_SIZE")

	rulerWidth := serviceWidth + composeWidth + statusWidth + actualWidth + 4 + getDisplayWidth("IMAGE_SIZE")
	fmt.Println(strings.Repeat("-", rulerWidth))

	for _, row := range rows {
		fmt.Printf(rowFormat, row.service, row.composeVersion, row.status, row.actualVersion, row.imageSize)
	}

	fmt.Println()
	fmt.Println("Legend:")
	fmt.Println("  COMPOSE_VERSION: Version specified in docker-compose.yaml")
	fmt.Println("  STATUS: Current container status (running/stopped/Not Running)")
	fmt.Println("  ACTUAL_VERSION: Version from running container image tag")
	fmt.Println("  IMAGE_SIZE: Size of the container image")
	fmt.Println("===============================================")
}

// checkImageExists checks if the specified "repository:tag" image exists locally.
func checkImageExists(imageName string) bool {
	_, exists := getLocalImageSizes()[strings.TrimSpace(imageName)]
	return exists
}

// parsePorts parses port information from docker compose ps output
func parsePorts(ports string) (string, string) {
	if ports == "" {
		return "-", "-"
	}

	// Parse multiple port mappings (e.g., "0.0.0.0:1024->1024/tcp, :::1024->1024/tcp")
	portMappings := strings.Split(ports, ",")
	if len(portMappings) > 0 {
		// Take the first port mapping
		firstMapping := strings.TrimSpace(portMappings[0])

		// Extract external and internal ports
		if strings.Contains(firstMapping, "->") {
			parts := strings.Split(firstMapping, "->")
			if len(parts) == 2 {
				external := strings.TrimSpace(parts[0])
				internal := strings.TrimSpace(parts[1])

				// Extract just the port number from external (e.g., "0.0.0.0:1024" -> "1024")
				if strings.Contains(external, ":") {
					externalParts := strings.Split(external, ":")
					external = externalParts[len(externalParts)-1]
				}

				// Extract just the port number from internal (e.g., "1024/tcp" -> "1024")
				if strings.Contains(internal, "/") {
					internalParts := strings.Split(internal, "/")
					internal = internalParts[0]
				}

				return internal, external
			}
		}
	}

	return "-", "-"
}

// humanTableColumns is the number of columns in the --human table.
const humanTableColumns = 7

// tableRow is one printed line of the --human table: seven cells, a horizontal
// rule drawn with the table's own border glyphs, or a category heading that
// spans the whole width.
type tableRow struct {
	cells     [humanTableColumns]string
	separator bool
	heading   string
}

// humanTableMinWidths are the column widths the table never shrinks below, so a
// short run of services still lines up with a longer one.
var humanTableMinWidths = [humanTableColumns]int{20, 12, 12, 8, 12, 12, 15}

// humanTableHeaders labels the columns.
var humanTableHeaders = [humanTableColumns]string{
	"SERVICE", "VERSION", "STATUS", "HEALTHY", "INTERNAL", "EXTERNAL", "IMAGE SIZE",
}

// buildTableRows lays the services out as printable rows.
//
// A service running a version other than the one compose names gets a second
// row — service column blank, version column naming what is really up — and the
// pair is fenced off with horizontal rules. Neighbouring mismatches share the
// rule between them instead of drawing two, and the rule that would sit against
// the header separator or the bottom border is dropped, since those already
// close the table off.
func buildTableRows(services []HumanServiceInfo) []tableRow {
	var rows []tableRow

	for _, service := range services {
		mismatch := service.ActualVersion != ""

		if mismatch && len(rows) > 0 && !rows[len(rows)-1].separator {
			rows = append(rows, tableRow{separator: true})
		}

		rows = append(rows, tableRow{cells: [humanTableColumns]string{
			service.Service,
			service.Version,
			service.Status,
			service.Healthy,
			service.InternalPort,
			service.ExternalPort,
			service.ImageSize,
		}})

		if mismatch {
			// Ports and size belong to the service, not to this one version, so
			// repeating them here would only add noise.
			rows = append(rows, tableRow{cells: [humanTableColumns]string{
				"",
				actualVersionPrefix + service.ActualVersion + " " + versionRunning,
				"",
				service.ActualHealthy,
				"",
				"",
				"",
			}})
			rows = append(rows, tableRow{separator: true})
		}
	}

	if n := len(rows); n > 0 && rows[n-1].separator {
		rows = rows[:n-1]
	}

	return rows
}

// groupedServices is one category's worth of rows for the --human table.
type groupedServices struct {
	category string
	icon     string
	services []HumanServiceInfo
}

// groupServicesByCategory buckets services into the display categories, in the
// order categoryDisplayOrder fixes, dropping categories nothing landed in.
//
// Within a category the services are sorted by name. The caller reads them out
// of a map in several places, and a map yields its keys in a different order on
// every run, so without this the table shuffled itself between invocations.
func groupServicesByCategory(services []HumanServiceInfo) []groupedServices {
	buckets := make(map[string][]HumanServiceInfo)
	for _, service := range services {
		category := categorizeService(service.Service, "")
		buckets[category] = append(buckets[category], service)
	}

	ordered := make([]groupedServices, 0, len(buckets))
	appendGroup := func(name, icon string) {
		group := buckets[name]
		if len(group) == 0 {
			return
		}
		sort.Slice(group, func(i, j int) bool { return group[i].Service < group[j].Service })
		ordered = append(ordered, groupedServices{category: name, icon: icon, services: group})
		delete(buckets, name)
	}

	for _, entry := range categoryDisplayOrder {
		appendGroup(entry.Name, entry.Icon)
	}

	// A category the ordered list does not know about still has to be shown;
	// dropping it would drop services from the table.
	leftovers := make([]string, 0, len(buckets))
	for name := range buckets {
		leftovers = append(leftovers, name)
	}
	sort.Strings(leftovers)
	for _, name := range leftovers {
		appendGroup(name, unknownCategoryIcon)
	}

	return ordered
}

// buildGroupedTableRows lays the services out grouped by category, each group
// introduced by a heading row.
//
// The per-group rows come from buildTableRows, which neither opens nor closes
// with a rule, so the rules drawn around a heading here cannot double up with
// the ones a version mismatch draws.
func buildGroupedTableRows(services []HumanServiceInfo) []tableRow {
	var rows []tableRow

	for _, group := range groupServicesByCategory(services) {
		if len(rows) > 0 {
			rows = append(rows, tableRow{separator: true})
		}
		rows = append(rows, tableRow{heading: group.icon + " " + group.category})
		rows = append(rows, tableRow{separator: true})
		rows = append(rows, buildTableRows(group.services)...)
	}

	return rows
}

// humanTableWidths measures each column, headers and every row included, so that
// the follow-up rows and their rules are as wide as the rest of the table.
//
// Heading rows are not measured per column: they span the whole table, so a long
// category name would otherwise stretch the SERVICE column and shift every row
// under it. Instead the last column is widened, once, if a heading would not
// fit — which leaves the columns where the content puts them.
func humanTableWidths(rows []tableRow) [humanTableColumns]int {
	widths := humanTableMinWidths

	for i, header := range humanTableHeaders {
		widths[i] = maxInt(widths[i], getDisplayWidth(header))
	}
	for _, row := range rows {
		if row.separator || row.heading != "" {
			continue
		}
		for i, cell := range row.cells {
			widths[i] = maxInt(widths[i], getDisplayWidth(cell))
		}
	}

	// Add some padding
	for i := range widths {
		widths[i] += 2
	}

	widest := 0
	for _, row := range rows {
		if row.heading != "" {
			widest = maxInt(widest, getDisplayWidth(row.heading))
		}
	}
	if inner := innerWidth(widths); widest > inner {
		widths[humanTableColumns-1] += widest - inner
	}

	return widths
}

// innerWidth is the space a heading row has between the outer borders: every
// column plus the interior separators it replaces.
func innerWidth(widths [humanTableColumns]int) int {
	total := humanTableColumns - 1
	for _, width := range widths {
		total += width
	}
	return total
}

// printTableBorder draws one horizontal line of the table with the given corner
// and junction glyphs.
func printTableBorder(left, mid, right string, widths [humanTableColumns]int) {
	segments := make([]string, 0, humanTableColumns)
	for _, width := range widths {
		segments = append(segments, strings.Repeat("─", width))
	}
	fmt.Printf("%s%s%s\n", left, strings.Join(segments, mid), right)
}

// printTableCells draws one content line, padding each cell to its column width.
//
// The padding is computed here rather than left to "%-*s", whose width counts
// runes. The column widths are measured in terminal columns, so a double-width
// rune would be padded one column too far and push the border out.
func printTableCells(cells [humanTableColumns]string, widths [humanTableColumns]int) {
	var line strings.Builder
	line.WriteString("│")
	for i, cell := range cells {
		line.WriteString(cell)
		line.WriteString(strings.Repeat(" ", maxInt(0, widths[i]-getDisplayWidth(cell))))
		line.WriteString("│")
	}
	fmt.Println(line.String())
}

// printTableHeading draws a category heading across the full width of the table.
//
// It spans the columns instead of sitting in the first one so that a category
// name longer than the SERVICE column cannot widen it, and the line still ends
// on the same terminal column as every other line.
func printTableHeading(heading string, widths [humanTableColumns]int) {
	padding := maxInt(0, innerWidth(widths)-getDisplayWidth(heading))
	fmt.Printf("│%s%s│\n", heading, strings.Repeat(" ", padding))
}

// displayServiceTable displays the service information in a formatted table
func displayServiceTable(services []HumanServiceInfo) {
	rows := buildGroupedTableRows(services)
	widths := humanTableWidths(rows)

	printTableBorder("┌", "┬", "┐", widths)
	printTableCells(humanTableHeaders, widths)
	printTableBorder("├", "┼", "┤", widths)

	for _, row := range rows {
		switch {
		case row.separator:
			printTableBorder("├", "┼", "┤", widths)
			continue
		case row.heading != "":
			printTableHeading(row.heading, widths)
			continue
		}
		printTableCells(row.cells, widths)
	}

	printTableBorder("└", "┴", "┘", widths)

	printVersionLegend()
	fmt.Println()
}

// printVersionLegend explains the marks the VERSION column uses.
func printVersionLegend() {
	fmt.Println("Legend:")
	fmt.Println("✓ Running on this version")
	fmt.Println("✗ Not running on this version (stopped, or image not installed locally)")
	fmt.Println("<- Version actually running, where it differs from docker-compose.yaml")
}

// maxInt returns the larger of two ints.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// getDisplayWidth calculates the display width of a string in terminal columns.
//
// Every branch of this function used to add 1, so it counted runes and the
// "accounting for Unicode" it claimed never happened — a name in Hangul or an
// emoji occupies two columns and pushed the table borders out by one each. The
// characters actually used in these tables (✓ ✗ -) stay at one column, since
// that is how terminals render them; only genuinely wide characters count two.
func getDisplayWidth(s string) int {
	width := 0
	// Width the previous rune contributed, so that an emoji presentation
	// selector can promote the character it follows.
	last := 0
	for _, r := range s {
		switch {
		case r == 0xFE0F:
			// U+FE0F asks for the emoji form of the preceding character, which
			// terminals draw double-width. "⚙️" is one column as U+2699 alone
			// and two once selected; without this the category headings that use
			// such icons measure one column short and their border drifts.
			if last == 1 {
				width++
				last = 2
			}
		case r == '✓' || r == '✗':
			// East Asian "ambiguous" width; terminals render these as one column.
			width++
			last = 1
		case r < 0x80:
			width++ // ASCII
			last = 1
		case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || r == 0x200B:
			// Combining marks and the zero-width space occupy no column.
			last = 0
		case isWideRune(r):
			width += 2
			last = 2
		default:
			width++
			last = 1
		}
	}
	return width
}

// isWideRune reports whether a rune is rendered double-width (East Asian Wide
// and Fullwidth, plus the emoji blocks used in this package's headings).
func isWideRune(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F: // Hangul Jamo
		return true
	case r >= 0x2E80 && r <= 0x303E: // CJK radicals, Kangxi, CJK symbols
		return true
	case r >= 0x3041 && r <= 0x33FF: // Hiragana, Katakana, Hangul Compat Jamo, CJK compat
		return true
	case r >= 0x3400 && r <= 0x4DBF: // CJK Unified Ideographs Extension A
		return true
	case r >= 0x4E00 && r <= 0x9FFF: // CJK Unified Ideographs
		return true
	case r >= 0xA000 && r <= 0xA4CF: // Yi
		return true
	case r >= 0xAC00 && r <= 0xD7A3: // Hangul syllables
		return true
	case r >= 0xF900 && r <= 0xFAFF: // CJK compatibility ideographs
		return true
	case r >= 0xFE30 && r <= 0xFE6F: // CJK compatibility forms
		return true
	case r >= 0xFF00 && r <= 0xFF60: // Fullwidth forms
		return true
	case r >= 0xFFE0 && r <= 0xFFE6: // Fullwidth signs
		return true
	case r >= 0x1F300 && r <= 0x1F64F: // Misc symbols and pictographs, emoticons
		return true
	case r >= 0x1F900 && r <= 0x1F9FF: // Supplemental symbols and pictographs
		return true
	case r >= 0x20000 && r <= 0x3FFFD: // CJK Unified Ideographs Extension B and later
		return true
	}
	return false
}

// displayServiceTableWithDependencies displays the service information with dependency categorization
// requestedList holds the already-resolved service names (see resolveServices);
// an empty list means no -s filter was given.
func displayServiceTableWithDependencies(services []HumanServiceInfo, requestedList []string) {
	if len(requestedList) == 0 {
		// No specific service requested, show all services in one table
		displayServiceTable(services)
		return
	}

	// Get dependency services (for reference, not used in categorization logic)
	_ = getDependencyServices(requestedList)

	// Categorize services
	var mainServices []HumanServiceInfo
	var depServices []HumanServiceInfo

	for _, service := range services {
		isMainService := false
		for _, reqService := range requestedList {
			if service.Service == reqService {
				isMainService = true
				break
			}
		}

		if isMainService {
			mainServices = append(mainServices, service)
		} else {
			depServices = append(depServices, service)
		}
	}

	// Display main services
	if len(mainServices) > 0 {
		fmt.Println("🎯 Requested Services:")
		displayServiceTable(mainServices)
	}

	// Display dependency services
	if len(depServices) > 0 {
		fmt.Println("📦 Dependency Services:")
		displayServiceTable(depServices)
	}
}

// getDependencyServices returns every service the given ones depend on,
// transitively, read from the compose file's depends_on.
//
// Two things changed here. The graph was a hardcoded literal that had to be
// edited by hand whenever compose changed, and it was walked only one level
// deep — `info --human -s cm-cicada` listed cm-beetle but not the cb-tumblebug
// that cm-beetle cannot start without. The walk below follows the whole chain
// and keeps a visited set, so a cycle in depends_on ends the traversal instead
// of looping forever.
//
// The services named in the argument are not returned as their own
// dependencies, even if the graph leads back to them; the caller lists those
// separately as the requested services.
func getDependencyServices(services []string) []string {
	parsed, err := loadComposeFile()
	if err != nil {
		return nil
	}

	requested := make(map[string]bool, len(services))
	for _, name := range services {
		requested[name] = true
	}

	visited := make(map[string]bool, len(parsed.Services))
	var dependencies []string

	var walk func(name string)
	walk = func(name string) {
		svc, exists := parsed.Services[name]
		if !exists {
			return
		}
		for _, dep := range svc.DependsOn {
			if visited[dep] {
				continue
			}
			visited[dep] = true

			if !requested[dep] {
				dependencies = append(dependencies, dep)
			}
			walk(dep)
		}
	}

	for _, name := range services {
		walk(name)
	}

	return dependencies
}

// removeDuplicateServices removes duplicate services from the slice
func removeDuplicateServices(services []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, service := range services {
		if !keys[service] {
			keys[service] = true
			result = append(result, service)
		}
	}

	return result
}
