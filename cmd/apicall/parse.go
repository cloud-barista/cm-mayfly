package apicall

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"
)

var swaggerFile string
var applyToYaml bool
var releaseVer string
var useLatest bool
var skipConfirm bool

// parseCmd is the `api tool` subcommand. It parses a Swagger JSON (file or URL)
// into api.yaml serviceActions and either prints them or, with --apply, writes
// them into conf/api.yaml.
var parseCmd = &cobra.Command{
	Use:   "tool",
	Short: "Swagger JSON parsing into api.yaml serviceActions",
	Long: `Parse a Swagger JSON (local file path or http(s) URL) into api.yaml serviceActions.

Without --apply it prints the generated serviceActions to stdout so you can
compose api.yaml manually (previous behavior).

With --apply it updates conf/api.yaml in place:
  --service <name>             replace that service's whole serviceActions (full dump)
  --service <name> --action X  update only the single action X of that service

A timestamped backup (api.yaml.bak.<ts>) is created first, and api.yaml is
restored automatically if the updated result fails to parse.

Examples:
  mayfly api tool -f ./cm-ant.swagger.json --service cm-ant
  mayfly api tool -f https://.../swagger.json --service cm-ant --apply
  mayfly api tool -f ./cm-ant.swagger.json --service cm-ant --action GetEstimateCost --apply`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTool(cmd)
	},
}

type swaggerAction struct {
	Method       string
	ResourcePath string
	Description  string
}

// swaggerTarget is one service to process with its resolved swagger source.
type swaggerTarget struct {
	svc string
	url string
}

const (
	hrThick = "==============================================="
	hrThin  = "---------------------------------------------------------------------------------"
)

func runTool(cmd *cobra.Command) error {
	// Load api.yaml so services.<svc>.swagger is available (the `api tool`
	// subcommand does not run the parent's config load).
	viper.SetConfigFile(configFile)
	_ = viper.ReadInConfig()

	fileSet := cmd.Flags().Changed("file")

	// Source options are mutually exclusive: -f / --latest / --release.
	srcN := 0
	for _, on := range []bool{fileSet, useLatest, releaseVer != ""} {
		if on {
			srcN++
		}
	}
	if srcN > 1 {
		return fmt.Errorf("소스 옵션은 -f / --latest / --release 중 하나만 지정하세요.")
	}

	// No source given: ask (latest vs a specific release).
	if srcN == 0 {
		if skipConfirm {
			return fmt.Errorf("자동화(-y) 시에는 소스(-f/--latest/--release)와 대상(--service)을 명시하세요.")
		}
		if promptChoice("Swagger 소스가 지정되지 않았습니다. 어떤 Swagger를 사용할까요?", "최신(각 서비스 기본 브랜치)", "특정 릴리스 버전") == 2 {
			releaseVer = promptLine("릴리스 버전을 입력하세요 (예: v0.5.2): ")
			if releaseVer == "" {
				return fmt.Errorf("릴리스 버전이 입력되지 않았습니다.")
			}
		} else {
			useLatest = true
		}
	}

	if !applyToYaml {
		fmt.Println("api.yaml에 직접 반영하려면 --apply 플래그를 지정해 주세요. (현재는 화면 출력만 합니다.)")
	}

	// Determine the target services.
	var svcs []string
	switch {
	case serviceName != "":
		svcs = []string{serviceName}
	case fileSet:
		return fmt.Errorf("-f로 단일 소스를 줄 때는 --service로 대상 서비스를 지정하세요.")
	default:
		if skipConfirm {
			return fmt.Errorf("자동화(-y) 시에는 --service를 명시하세요.")
		}
		if promptChoice("--service 미지정 — 처리 대상을 선택하세요.", "특정 서비스만", "전체 서비스") == 1 {
			s := promptLine("서비스명을 입력하세요: ")
			if s == "" {
				return fmt.Errorf("서비스명이 입력되지 않았습니다.")
			}
			svcs = []string{s}
		} else {
			svcs = registeredSwaggerServices()
			if len(svcs) == 0 {
				return fmt.Errorf("swagger 정보가 등록된 서비스가 없습니다 (api.yaml services.<svc>.swagger).")
			}
		}
	}

	// Resolve each service's source URL; for registry sources, check existence.
	var present, missing []swaggerTarget
	for _, svc := range svcs {
		if fileSet {
			present = append(present, swaggerTarget{svc, swaggerFile})
			continue
		}
		url, ok := resolveSwaggerURL(svc)
		if !ok {
			if len(svcs) == 1 {
				return fmt.Errorf("서비스 %q에 swagger 정보가 없습니다. -f로 직접 지정하거나 api.yaml의 services.%s.swagger를 확인하세요.", svc, svc)
			}
			missing = append(missing, swaggerTarget{svc, "(swagger 미등록)"})
			continue
		}
		if swaggerExists(url) {
			present = append(present, swaggerTarget{svc, url})
		} else {
			missing = append(missing, swaggerTarget{svc, url})
		}
	}

	// Summary + confirmation.
	verb := "api.yaml 구조로 화면에 출력합니다"
	if applyToYaml {
		verb = "api.yaml에 반영합니다"
	}
	multi := len(svcs) > 1
	if multi {
		relLabel := "최신"
		if releaseVer != "" {
			relLabel = releaseVer + " 릴리스 버전의"
		}
		fmt.Printf("\n모든 서비스에 대해 %s API를 %s.\n", relLabel, verb)
		if len(missing) > 0 {
			fmt.Println()
			fmt.Println(hrThick)
			fmt.Println("[경고]")
			fmt.Println(hrThick)
			fmt.Println("아래 서비스들은 요청한 버전의 API 문서가 존재하지 않습니다. 버전 및 URL을 확인하세요.")
			fmt.Println("(-f 와 -s 옵션을 이용하면 특정 URL과 특정 서비스를 지정할 수 있습니다.)")
			printTargets(missing)
			fmt.Println(hrThin)
		}
		if len(present) == 0 {
			fmt.Println("\n처리할 수 있는 서비스가 없습니다.")
			return nil
		}
		fmt.Println("\n아래 서비스들의 API만 처리합니다.")
		printTargets(present)
	} else {
		if len(present) == 0 {
			m := missing[0]
			return fmt.Errorf("요청한 Swagger를 찾을 수 없습니다: %s : %s — 버전(vX.Y.Z) 또는 --latest 를 확인하세요.", m.svc, m.url)
		}
		t := present[0]
		scope := t.svc + " Service 전체"
		if actionName != "" {
			scope = fmt.Sprintf("%s Service의 %s 액션", t.svc, actionName)
		}
		fmt.Printf("\n%s에 대해 %s.\n소스: %s\n", scope, verb, t.url)
	}

	// A raw -f source carries no requested version, so the version recorded in
	// api.yaml falls back to whatever the document declares in info.version —
	// usually "latest", which does not say which release is actually deployed.
	// Warn before writing that, because the only fix afterwards is editing
	// api.yaml by hand.
	if applyToYaml && fileSet {
		fmt.Println("\n⚠ Caution: the Swagger given with -f does not carry the release it belongs to.")
		fmt.Println("  The version recorded in api.yaml will be the document's own info.version")
		fmt.Println("  (most services publish \"latest\"), so it may not match the release you are")
		fmt.Println("  deploying, and correcting it afterwards means editing api.yaml by hand.")
		fmt.Println("  Use --release <tag> instead to record the exact version.")
	}

	if !skipConfirm && !confirmYN("\n계속 진행하시겠습니까? (Y/n): ") {
		fmt.Println("취소되었습니다.")
		return nil
	}

	var firstErr error
	for _, t := range present {
		if err := processOne(t.svc, t.url); err != nil {
			fmt.Printf("[%s] 실패: %v\n", t.svc, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// printTargets prints "  <svc> : <url>" with the service names left-aligned.
func printTargets(items []swaggerTarget) {
	w := 0
	for _, t := range items {
		if len(t.svc) > w {
			w = len(t.svc)
		}
	}
	for _, t := range items {
		fmt.Printf("  %-*s : %s\n", w, t.svc, t.url)
	}
}

// processOne reads one swagger source and prints or applies it for one service.
func processOne(svc, source string) error {
	data, err := readSwaggerSource(source)
	if err != nil {
		return fmt.Errorf("failed to read swagger %q: %w", source, err)
	}
	json := string(data)
	actions := extractActions(json)
	if actionName != "" {
		key := convertActionlName(actionName)
		picked, ok := actions[key]
		if !ok {
			return fmt.Errorf("action %q (normalized to %q) not found in the swagger", actionName, key)
		}
		actions = map[string]swaggerAction{key: picked}
	}
	version := recordedVersion(gjson.Get(json, "info.version").String())
	if !applyToYaml {
		fmt.Printf("\n# %s  (version=%s)\n", svc, version)
		fmt.Print(renderActions(actions))
		return nil
	}
	apiFile, err := resolveApiFile()
	if err != nil {
		return err
	}
	return applyToApiYaml(apiFile, svc, actionName != "", actions, version)
}

// resolveApiFile returns the api.yaml that --apply should update.
//
// -c names the file the command works on, and it has to name it for writing as
// well as for reading: resolving the swagger URLs from one file and then writing
// the result into another is how an update silently lands in the wrong place.
//
// Without -c the configured default is used when it exists, and otherwise an
// api.yaml in the current directory is accepted, so the tool still works from a
// checkout whose layout puts the file elsewhere.
func resolveApiFile() (string, error) {
	if configFile != "" {
		if _, err := os.Stat(configFile); err == nil {
			return configFile, nil
		}
		// An explicitly named file that does not exist is an error, not a
		// reason to quietly fall back to some other file.
		if configFile != common.API_FILE {
			return "", fmt.Errorf("config file not found: %s", configFile)
		}
	}
	for _, candidate := range []string{common.API_FILE, "./api.yaml"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("api.yaml not found (looked for %s and ./api.yaml)\nRun this from the directory that holds it, or point at it with -c", common.API_FILE)
}

// recordedVersion returns the version recorded to api.yaml as
// "<requested version>(<swagger info.version>)".
//
// The leading part is what the option asked for — the tag from --release (with
// the leading "v" stripped, since we record versions without it), or "latest"
// for --latest. The parenthetical is exactly what the swagger document declares
// in its info.version, taken verbatim; only a missing/blank value is recorded as
// "None" to flag that the document carries no version:
//
//	--release 0.12.22, document info.version="latest" -> "0.12.22(latest)"
//	--release 0.6.0,   document info.version="0.5.0"  -> "0.6.0(0.5.0)"
//	--latest,          document info.version="0.6.0"  -> "latest(0.6.0)"
//	--latest,          document info.version="latest" -> "latest(latest)"
//	--latest,          document info.version=""       -> "latest(None)"
//
// A raw -f source has no requested version, so it keeps the document's own
// value (or "None" when the document declares none).
func recordedVersion(infoVer string) string {
	inner := strings.TrimSpace(infoVer)
	if inner == "" {
		inner = "None"
	}
	switch {
	case releaseVer != "":
		return strings.TrimPrefix(releaseVer, "v") + "(" + inner + ")"
	case useLatest:
		return "latest(" + inner + ")"
	default:
		return inner
	}
}

// releaseURLTag returns the tag embedded in a release swagger URL. Cloud-Barista
// tags are "v"-prefixed, so normalize to that regardless of whether the user
// passed the tag with or without the leading "v".
func releaseURLTag() string {
	return "v" + strings.TrimPrefix(releaseVer, "v")
}

// resolveSwaggerURL returns the swagger URL for svc from api.yaml's
// services.<svc>.swagger (latest, or release with {release} substituted).
func resolveSwaggerURL(svc string) (string, bool) {
	if releaseVer != "" {
		rel := viper.GetString("services." + svc + ".swagger.release")
		if rel == "" {
			return "", false
		}
		return strings.ReplaceAll(rel, "{release}", releaseURLTag()), true
	}
	latest := viper.GetString("services." + svc + ".swagger.latest")
	if latest == "" {
		return "", false
	}
	return latest, true
}

// swaggerExists reports whether the swagger URL responds with 200. It goes
// through common.NewHTTPClient so an unreachable host fails on the timeout
// instead of blocking forever.
func swaggerExists(url string) bool {
	resp, err := common.NewHTTPClient().R().Get(url) // #nosec G107 -- registry URL from api.yaml
	if err != nil {
		return false
	}
	return resp.StatusCode() == http.StatusOK
}

// registeredSwaggerServices returns the services that have a swagger entry.
func registeredSwaggerServices() []string {
	var out []string
	for svc := range viper.GetStringMap("services") {
		if viper.IsSet("services." + svc + ".swagger") {
			out = append(out, svc)
		}
	}
	sort.Strings(out)
	return out
}

// stdinReader is shared across prompts so buffered input is not lost between
// successive reads.
var stdinReader = bufio.NewReader(os.Stdin)

func promptChoice(q string, opts ...string) int {
	fmt.Println(q)
	for i, o := range opts {
		fmt.Printf("  %d) %s\n", i+1, o)
	}
	fmt.Print("선택: ")
	line, _ := stdinReader.ReadString('\n')
	n, _ := strconv.Atoi(strings.TrimSpace(line))
	if n < 1 || n > len(opts) {
		return 1
	}
	return n
}

func promptLine(prompt string) string {
	fmt.Print(prompt)
	line, _ := stdinReader.ReadString('\n')
	return strings.TrimSpace(line)
}

// confirmYN reads a yes/no answer. Enter (empty) defaults to yes. An EOF (no
// input available) is treated as "no" so non-interactive runs do not proceed
// unintentionally; use -y to skip the prompt in automation.
func confirmYN(prompt string) bool {
	fmt.Print(prompt)
	line, err := stdinReader.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	s := strings.ToLower(strings.TrimSpace(line))
	return s == "" || s == "y" || s == "yes"
}

// readSwaggerSource reads a swagger document from a local file or an http(s) URL.
func readSwaggerSource(src string) ([]byte, error) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		resp, err := common.NewHTTPClient().R().Get(src) // #nosec G107 -- src is an operator-supplied swagger URL
		if err != nil {
			return nil, err
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("GET %s returned status %d", src, resp.StatusCode())
		}
		return resp.Body(), nil
	}
	return os.ReadFile(src) // #nosec G304 -- src is an operator-supplied swagger path
}

// extractActions builds operationId -> action from a swagger document. Operations
// without an operationId are skipped (they cannot become a named api.yaml action).
func extractActions(json string) map[string]swaggerAction {
	out := map[string]swaggerAction{}
	for path, methods := range gjson.Get(json, "paths").Map() {
		for method, details := range methods.Map() {
			if strings.ToLower(method) == "parameters" {
				continue
			}
			opID := details.Get("operationId").String()
			if opID == "" {
				continue
			}
			out[convertActionlName(opID)] = swaggerAction{
				Method:       method,
				ResourcePath: path,
				Description:  details.Get("description").String(),
			}
		}
	}
	return out
}

// renderActions renders actions as the api.yaml serviceActions body that sits
// under "  <service>:" (4-space indent), in deterministic (sorted) order so the
// generated text is stable across runs.
func renderActions(actions map[string]swaggerAction) string {
	names := make([]string, 0, len(actions))
	for n := range actions {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		a := actions[n]
		fmt.Fprintf(&b, "    %s:\n", n)
		fmt.Fprintf(&b, "      method: %s\n", a.Method)
		fmt.Fprintf(&b, "      resourcePath: %s\n", a.ResourcePath)
		fmt.Fprintf(&b, "      description: %q\n", a.Description)
	}
	return b.String()
}

// applyToApiYaml writes the generated actions into apiFile (conf/api.yaml) with a
// timestamped backup, then verifies the result still parses as YAML and restores
// the original on failure.
func applyToApiYaml(apiFile, service string, singleAction bool, actions map[string]swaggerAction, version string) error {
	orig, err := os.ReadFile(apiFile) // #nosec G304 -- fixed internal api.yaml path
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", apiFile, err)
	}

	// apiFile is whatever resolveApiFile settled on — the configured default or
	// the file named with -c — so the backup sits next to the file being
	// updated rather than next to a fixed path.
	backup := fmt.Sprintf("%s.bak.%s", apiFile, time.Now().Format("20060102-150405"))
	if err := os.WriteFile(backup, orig, 0600); err != nil { // #nosec G703 -- derived from the fixed internal api.yaml path
		return fmt.Errorf("failed to write backup %s: %w", backup, err)
	}

	updated, err := updateServiceActionsBlock(string(orig), service, singleAction, actions)
	if err != nil {
		return err
	}
	// On a full-service dump, also sync services.<svc>.version to the swagger's
	// version so the recorded version matches the applied spec (otherwise the
	// version and the actions can drift). A single --action is a partial patch,
	// so it leaves the service version untouched.
	versionSynced := false
	if !singleAction && version != "" {
		if u, ok := updateServiceVersion(updated, service, version); ok {
			updated = u
			versionSynced = true
		}
	}

	if err := os.WriteFile(apiFile, []byte(updated), 0600); err != nil { // #nosec G703 -- fixed internal api.yaml path
		return fmt.Errorf("failed to write %s: %w", apiFile, err)
	}

	if verr := verifyApiYaml(apiFile); verr != nil {
		_ = os.WriteFile(apiFile, orig, 0600) // #nosec G703 -- fixed internal api.yaml path; restores the original on failure
		return fmt.Errorf("updated %s failed verification (%v); restored original (backup kept at %s)", apiFile, verr, backup)
	}

	scope := fmt.Sprintf("all serviceActions (%d)", len(actions))
	if singleAction {
		scope = "1 action"
	}
	if versionSynced {
		scope = fmt.Sprintf("%s + version %s", scope, version)
	}
	fmt.Printf("Applied %s for service %q to %s; backup: %s\n", scope, service, apiFile, backup)
	return nil
}

// isServiceHeaderLine reports whether line is the "  <service>:" header under
// services:, tolerating a trailing inline comment (e.g. the api.yaml ships
// "  cb-spider: #service name"). Matching on the exact string would miss such a
// header and silently skip its version sync.
// yamlRoot parses content and returns the top-level mapping node, so callers can
// locate sections and keys by structure — and by the exact line numbers the
// parser reports — instead of by fragile string matching. Only the located lines
// are spliced by the callers, so comments, blank lines, ${ENV} placeholders and
// emoji elsewhere in the file are left byte-for-byte intact.
func yamlRoot(content string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("empty YAML document")
	}
	return doc.Content[0], nil
}

// mapChild returns the value node for key in a mapping node, or nil.
func mapChild(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// keyEntryLines returns the 1-based line of key's entry in mapping m and the
// 1-based line of the following sibling entry (nextLine == 0 when key is the last
// entry). A returned line of 0 means the key is absent.
func keyEntryLines(m *yaml.Node, key string) (line, nextLine int) {
	if m == nil || m.Kind != yaml.MappingNode {
		return 0, 0
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			line = m.Content[i].Line
			if i+2 < len(m.Content) {
				nextLine = m.Content[i+2].Line
			}
			return line, nextLine
		}
	}
	return 0, 0
}

// updateServiceVersion sets services.<service>.version to version, editing only
// that one line (the rest of the file is preserved). It returns ok=false if the
// service or its version line is not found, leaving the content unchanged.
func updateServiceVersion(content, service, version string) (string, bool) {
	root, err := yamlRoot(content)
	if err != nil {
		return content, false
	}
	vLine, _ := keyEntryLines(mapChild(mapChild(root, "services"), service), "version")
	if vLine == 0 {
		return content, false
	}
	lines := strings.Split(content, "\n")
	idx := vLine - 1
	if idx < 0 || idx >= len(lines) {
		return content, false
	}
	indent := lines[idx][:len(lines[idx])-len(strings.TrimLeft(lines[idx], " "))]
	lines[idx] = indent + "version: " + version
	return strings.Join(lines, "\n"), true
}

// verifyApiYaml re-reads api.yaml and ensures it still parses as YAML.
func verifyApiYaml(apiFile string) error {
	data, err := os.ReadFile(apiFile) // #nosec G304 -- fixed internal api.yaml path
	if err != nil {
		return err
	}
	// General YAML validity.
	var v map[string]interface{}
	if err := yaml.Unmarshal(data, &v); err != nil {
		return err
	}
	// serviceActions structure: each serviceActions.<svc>.<action> must be a
	// mapping (method/resourcePath/description), not a scalar. A stray scalar —
	// e.g. a version line misplaced under serviceActions — is valid YAML but
	// breaks consumers that load api.yaml into typed structs (the console proxy),
	// so reject it here instead of writing a structurally broken catalog.
	var typed struct {
		ServiceActions map[string]map[string]map[string]interface{} `yaml:"serviceActions"`
	}
	if err := yaml.Unmarshal(data, &typed); err != nil {
		return fmt.Errorf("serviceActions structure invalid — an action entry is not a mapping (stray scalar under serviceActions): %w", err)
	}
	return nil
}

// updateServiceActionsBlock edits api.yaml text so the rest of the file (services,
// comments, ${ENV} placeholders, other services' actions) is preserved. It either
// replaces the whole "  <service>:" block under the top-level "serviceActions:"
// map (full dump) or a single "    <action>:" entry within it.
func updateServiceActionsBlock(content, service string, singleAction bool, actions map[string]swaggerAction) (string, error) {
	root, err := yamlRoot(content)
	if err != nil {
		return "", err
	}
	sa := mapChild(root, "serviceActions")
	if sa == nil {
		return "", fmt.Errorf("'serviceActions:' section not found in the target api.yaml")
	}
	lines := strings.Split(content, "\n")

	// End of the serviceActions section (1-based, exclusive): the next top-level
	// key's line, or EOF when serviceActions is the last top-level key.
	saEnd := len(lines) + 1
	if _, saNextTop := keyEntryLines(root, "serviceActions"); saNextTop != 0 {
		saEnd = saNextTop
	}

	newSvcBody := renderActions(actions) // 4-space-indented action blocks

	svcKeyLine, svcNextLine := keyEntryLines(sa, service)
	if svcKeyLine == 0 {
		// Service absent: append a new "  <service>:" block at the end of the section.
		at := saEnd - 1 // 0-based insert index
		block := append([]string{"  " + service + ":"}, splitNonEmpty(newSvcBody)...)
		out := append([]string{}, lines[:at]...)
		out = append(out, block...)
		out = append(out, lines[at:]...)
		return strings.Join(out, "\n"), nil
	}

	svcStart := svcKeyLine - 1 // 0-based "  <service>:" line
	svcEnd := saEnd - 1        // 0-based, exclusive
	if svcNextLine != 0 {
		svcEnd = svcNextLine - 1
	}

	if !singleAction {
		// Replace the whole service body with the generated actions.
		out := append([]string{}, lines[:svcStart+1]...)
		out = append(out, splitNonEmpty(newSvcBody)...)
		out = append(out, lines[svcEnd:]...)
		return strings.Join(out, "\n"), nil
	}

	// Single action: actions has exactly one entry.
	var actName string
	for n := range actions {
		actName = n
	}
	newActBody := splitNonEmpty(renderActions(actions))

	actKeyLine, actNextLine := keyEntryLines(mapChild(sa, service), actName)
	if actKeyLine == 0 {
		// Action absent: insert at the end of the service block.
		out := append([]string{}, lines[:svcEnd]...)
		out = append(out, newActBody...)
		out = append(out, lines[svcEnd:]...)
		return strings.Join(out, "\n"), nil
	}
	actStart := actKeyLine - 1
	actEnd := svcEnd
	if actNextLine != 0 {
		actEnd = actNextLine - 1
	}
	out := append([]string{}, lines[:actStart]...)
	out = append(out, newActBody...)
	out = append(out, lines[actEnd:]...)
	return strings.Join(out, "\n"), nil
}

// splitNonEmpty splits a rendered block into lines, dropping the trailing empty
// element that a "...\n" string produces.
func splitNonEmpty(block string) []string {
	if block == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(block, "\n"), "\n")
}

func convertActionlName(tmpActionName string) string {
	//일부 특수 기호들 제거
	tmpActionName = strings.ReplaceAll(tmpActionName, ":", "-")
	tmpActionName = strings.ReplaceAll(tmpActionName, "`", "")
	tmpActionName = strings.ReplaceAll(tmpActionName, "'", "")

	//카멜타입으로 변경
	tmpActionName = toCamelCase(tmpActionName)

	return tmpActionName
}

func toCamelCase(str string) string {
	words := strings.Fields(str) // 문자열을 공백을 기준으로 단어로 분할
	var result strings.Builder
	for _, word := range words {
		result.WriteString(strings.Title(word)) // 각 단어의 첫 글자를 대문자로 만듦
	}
	return result.String()
}

func init() {
	apiCmd.AddCommand(parseCmd)
	parseCmd.PersistentFlags().StringVarP(&swaggerFile, "file", "f", common.SWAG_FILE, "Swagger JSON source: local file path or http(s) URL")
	parseCmd.PersistentFlags().BoolVar(&applyToYaml, "apply", false, "Apply parsed actions into conf/api.yaml (default: print to stdout)")
	parseCmd.PersistentFlags().BoolVar(&useLatest, "latest", false, "Use each service's latest swagger URL from api.yaml (services.<svc>.swagger.latest)")
	parseCmd.PersistentFlags().StringVar(&releaseVer, "release", "", "Use a specific release tag's swagger (services.<svc>.swagger.release, {release} substituted)")
	parseCmd.PersistentFlags().BoolVarP(&skipConfirm, "yes", "y", false, "Skip the confirmation prompt (for automation; requires source and target to be specified)")
}
