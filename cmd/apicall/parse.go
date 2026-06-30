package apicall

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v2"
)

var swaggerFile string
var applyToYaml bool

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
		return runTool()
	},
}

type swaggerAction struct {
	Method       string
	ResourcePath string
	Description  string
}

func runTool() error {
	data, err := readSwaggerSource(swaggerFile)
	if err != nil {
		return fmt.Errorf("failed to read swagger source %q: %w", swaggerFile, err)
	}
	json := string(data)
	actions := extractActions(json)

	// Optional single-action filter.
	if actionName != "" {
		key := convertActionlName(actionName)
		picked, ok := actions[key]
		if !ok {
			return fmt.Errorf("action %q (normalized to %q) not found in the swagger", actionName, key)
		}
		actions = map[string]swaggerAction{key: picked}
	}

	if !applyToYaml {
		// Print mode: assist manual api.yaml editing (previous behavior).
		fmt.Println("API Title:", gjson.Get(json, "info.title").String())
		fmt.Println("API Version:", gjson.Get(json, "info.version").String())
		fmt.Println("Host:", gjson.Get(json, "host").String())
		fmt.Println("Base Path:", gjson.Get(json, "basePath").String())
		fmt.Print(renderActions(actions))
		return nil
	}

	if serviceName == "" {
		return fmt.Errorf("--apply requires --service to choose which service's serviceActions to update")
	}
	version := gjson.Get(json, "info.version").String()
	return applyToApiYaml(common.API_FILE, serviceName, actionName != "", actions, version)
}

// readSwaggerSource reads a swagger document from a local file or an http(s) URL.
func readSwaggerSource(src string) ([]byte, error) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		resp, err := http.Get(src) // #nosec G107 -- src is an operator-supplied swagger URL
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GET %s returned status %d", src, resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
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

	backup := fmt.Sprintf("%s.bak.%s", apiFile, time.Now().Format("20060102-150405"))
	if err := os.WriteFile(backup, orig, 0600); err != nil {
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

	if err := os.WriteFile(apiFile, []byte(updated), 0600); err != nil {
		return fmt.Errorf("failed to write %s: %w", apiFile, err)
	}

	if verr := verifyApiYaml(apiFile); verr != nil {
		_ = os.WriteFile(apiFile, orig, 0600) // restore original on failure
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

// updateServiceVersion sets services.<service>.version to version (text edit, so
// the rest of the file is preserved). It returns ok=false if the service or its
// version line is not found, leaving the content unchanged.
func updateServiceVersion(content, service, version string) (string, bool) {
	lines := strings.Split(content, "\n")

	secStart := -1
	for i, l := range lines {
		if strings.TrimRight(l, " ") == "services:" {
			secStart = i
			break
		}
	}
	if secStart < 0 {
		return content, false
	}
	secEnd := len(lines)
	for i := secStart + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if !strings.HasPrefix(lines[i], " ") {
			secEnd = i
			break
		}
	}

	svcHeader := "  " + service + ":"
	svcStart := -1
	for i := secStart + 1; i < secEnd; i++ {
		if strings.TrimRight(lines[i], " ") == svcHeader {
			svcStart = i
			break
		}
	}
	if svcStart < 0 {
		return content, false
	}
	svcEnd := secEnd
	for i := svcStart + 1; i < secEnd; i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if strings.HasPrefix(lines[i], "  ") && !strings.HasPrefix(lines[i], "   ") {
			svcEnd = i
			break
		}
	}
	for i := svcStart + 1; i < svcEnd; i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "version:") {
			lines[i] = "    version: " + version
			return strings.Join(lines, "\n"), true
		}
	}
	return content, false
}

// verifyApiYaml re-reads api.yaml and ensures it still parses as YAML.
func verifyApiYaml(apiFile string) error {
	data, err := os.ReadFile(apiFile) // #nosec G304 -- fixed internal api.yaml path
	if err != nil {
		return err
	}
	var v map[string]interface{}
	return yaml.Unmarshal(data, &v)
}

// updateServiceActionsBlock edits api.yaml text so the rest of the file (services,
// comments, ${ENV} placeholders, other services' actions) is preserved. It either
// replaces the whole "  <service>:" block under the top-level "serviceActions:"
// map (full dump) or a single "    <action>:" entry within it.
func updateServiceActionsBlock(content, service string, singleAction bool, actions map[string]swaggerAction) (string, error) {
	lines := strings.Split(content, "\n")

	// Locate top-level "serviceActions:".
	saStart := -1
	for i, l := range lines {
		if strings.TrimRight(l, " ") == "serviceActions:" {
			saStart = i
			break
		}
	}
	if saStart < 0 {
		return "", fmt.Errorf("'serviceActions:' section not found in %s", common.API_FILE)
	}
	// End of the serviceActions section = next non-indented, non-empty line.
	saEnd := len(lines)
	for i := saStart + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if !strings.HasPrefix(lines[i], " ") {
			saEnd = i
			break
		}
	}

	svcHeader := "  " + service + ":"
	newSvcBody := renderActions(actions) // 4-space-indented action blocks

	// Locate "  <service>:" within the serviceActions section.
	svcStart := -1
	for i := saStart + 1; i < saEnd; i++ {
		if strings.TrimRight(lines[i], " ") == svcHeader {
			svcStart = i
			break
		}
	}

	if svcStart < 0 {
		// Service absent: append a new "  <service>:" block at the end of the section.
		block := []string{svcHeader}
		block = append(block, splitNonEmpty(newSvcBody)...)
		out := append([]string{}, lines[:saEnd]...)
		out = append(out, block...)
		out = append(out, lines[saEnd:]...)
		return strings.Join(out, "\n"), nil
	}

	// Service block body ends at the next 2-space key (next service) or section end.
	svcEnd := saEnd
	for i := svcStart + 1; i < saEnd; i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if strings.HasPrefix(lines[i], "  ") && !strings.HasPrefix(lines[i], "   ") {
			svcEnd = i
			break
		}
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
	actHeader := "    " + actName + ":"
	newActBody := splitNonEmpty(renderActions(actions))

	// Locate "    <action>:" within the service block.
	actStart := -1
	for i := svcStart + 1; i < svcEnd; i++ {
		if strings.TrimRight(lines[i], " ") == actHeader {
			actStart = i
			break
		}
	}
	if actStart < 0 {
		// Action absent: insert at the end of the service block.
		out := append([]string{}, lines[:svcEnd]...)
		out = append(out, newActBody...)
		out = append(out, lines[svcEnd:]...)
		return strings.Join(out, "\n"), nil
	}
	// Action body ends at the next 4-space key or the service block end.
	actEnd := svcEnd
	for i := actStart + 1; i < svcEnd; i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if strings.HasPrefix(lines[i], "    ") && !strings.HasPrefix(lines[i], "     ") {
			actEnd = i
			break
		}
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
}
