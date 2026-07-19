package docker

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn and returns everything it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to open a pipe: %v", err)
	}

	prev := os.Stdout
	os.Stdout = write
	defer func() { os.Stdout = prev }()

	fn()

	if err := write.Close(); err != nil {
		t.Fatalf("failed to close the pipe: %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, read); err != nil {
		t.Fatalf("failed to read the captured output: %v", err)
	}
	return buf.String()
}

// matching builds a service row whose running version is the compose one.
func matching(name string) HumanServiceInfo {
	return HumanServiceInfo{
		Service: name, Version: "1.0.0 ✓", Status: "running", Healthy: "✓",
		InternalPort: "8080", ExternalPort: "8080", ImageSize: "10MB",
	}
}

// mismatched builds a service row running a version other than the compose one.
func mismatched(name, actual string) HumanServiceInfo {
	return HumanServiceInfo{
		Service: name, Version: "1.0.0 ✗", Status: "running", Healthy: "✓",
		InternalPort: "8080", ExternalPort: "8080", ImageSize: "10MB",
		ActualVersion: actual, ActualHealthy: "✓",
	}
}

// A service running the version compose names is one plain row, with no rules
// around it and nothing added underneath.
func TestBuildTableRowsMatchingVersionIsOneRow(t *testing.T) {
	rows := buildTableRows([]HumanServiceInfo{matching("cb-spider")})

	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(rows), rows)
	}
	if rows[0].separator {
		t.Error("the row is a separator, want cells")
	}
	if rows[0].cells[1] != "1.0.0 ✓" {
		t.Errorf("version cell = %q, want %q", rows[0].cells[1], "1.0.0 ✓")
	}
}

// A mismatch adds a follow-up row naming what is really running, and fences the
// pair off. The follow-up carries only the version and the health; ports and
// size belong to the service, not to that one version.
func TestBuildTableRowsMismatchAddsFencedFollowUp(t *testing.T) {
	rows := buildTableRows([]HumanServiceInfo{
		matching("cb-spider"),
		mismatched("cb-tumblebug", "0.12.02"),
		matching("cm-ant"),
	})

	want := []string{
		"cb-spider",
		"---",
		"cb-tumblebug",
		"", // the follow-up row
		"---",
		"cm-ant",
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(rows), len(want), rows)
	}
	for i, w := range want {
		if w == "---" {
			if !rows[i].separator {
				t.Errorf("row %d is not a separator, want one", i)
			}
			continue
		}
		if rows[i].separator {
			t.Fatalf("row %d is a separator, want the %q row", i, w)
		}
		if rows[i].cells[0] != w {
			t.Errorf("row %d service = %q, want %q", i, rows[i].cells[0], w)
		}
	}

	followUp := rows[3].cells
	if followUp[1] != "<- 0.12.02 ✓" {
		t.Errorf("follow-up version = %q, want %q", followUp[1], "<- 0.12.02 ✓")
	}
	if followUp[3] != "✓" {
		t.Errorf("follow-up healthy = %q, want %q", followUp[3], "✓")
	}
	for _, i := range []int{0, 2, 4, 5, 6} {
		if followUp[i] != "" {
			t.Errorf("follow-up cell %d = %q, want it blank", i, followUp[i])
		}
	}
}

// Back-to-back mismatches share the rule between them rather than drawing two,
// and no rule is left against the bottom border, which already closes the table.
func TestBuildTableRowsCollapsesAdjacentSeparators(t *testing.T) {
	rows := buildTableRows([]HumanServiceInfo{
		matching("cb-spider"),
		mismatched("a", "0.1"),
		mismatched("b", "0.2"),
		mismatched("c", "0.3"),
	})

	for i := 1; i < len(rows); i++ {
		if rows[i].separator && rows[i-1].separator {
			t.Errorf("rows %d and %d are both separators", i-1, i)
		}
	}
	if last := rows[len(rows)-1]; last.separator {
		t.Error("the table ends with a separator, want the bottom border to close it")
	}
	// cb-spider, rule, then three (row, follow-up) pairs separated by two rules.
	if len(rows) != 1+1+3*2+2 {
		t.Fatalf("got %d rows, want 10: %+v", len(rows), rows)
	}
}

// A mismatch on the very first service needs no leading rule: the header
// separator right above it already rules the block off.
func TestBuildTableRowsNoLeadingSeparator(t *testing.T) {
	rows := buildTableRows([]HumanServiceInfo{mismatched("cb-tumblebug", "0.12.02")})

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(rows), rows)
	}
	if rows[0].separator {
		t.Error("the table opens with a separator, want the header separator to serve")
	}
}

// The follow-up row is measured like any other, so a version long enough to be
// the widest cell in the table still fits between the borders.
func TestHumanTableWidthsCountFollowUpRows(t *testing.T) {
	long := "0.12.02-with-a-very-long-suffix"
	rows := buildTableRows([]HumanServiceInfo{mismatched("cb-tumblebug", long)})

	widths := humanTableWidths(rows)
	want := getDisplayWidth(actualVersionPrefix+long+" ✓") + 2
	if widths[1] != want {
		t.Errorf("version column width = %d, want %d", widths[1], want)
	}
}

// Every printed line is the same width in terminal columns, follow-up rows and
// rules included. Cells are padded by display width rather than by the rune
// count a "%-*s" verb would use, so a double-width glyph cannot push a border
// out by one.
func TestDisplayServiceTableLinesAlign(t *testing.T) {
	out := captureStdout(t, func() {
		displayServiceTable([]HumanServiceInfo{
			matching("cb-spider"),
			mismatched("cb-tumblebug", "0.12.02"),
			{Service: "cm-ant", Version: "0.5.4 ✗", Status: "Not Found",
				Healthy: "-", InternalPort: "-", ExternalPort: "-", ImageSize: "-"},
		})
	})

	var width int
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "│") && !strings.HasPrefix(line, "┌") &&
			!strings.HasPrefix(line, "├") && !strings.HasPrefix(line, "└") {
			continue
		}
		got := getDisplayWidth(line)
		if width == 0 {
			width = got
			continue
		}
		if got != width {
			t.Errorf("line %q is %d columns wide, want %d", line, got, width)
		}
	}
	if width == 0 {
		t.Fatal("no table lines were printed")
	}
}

// The legend explains the marks the version column now carries.
func TestDisplayServiceTablePrintsLegend(t *testing.T) {
	out := captureStdout(t, func() {
		displayServiceTable([]HumanServiceInfo{matching("cb-spider")})
	})

	for _, want := range []string{"Legend:", "✓ Running on this version", "<- Version actually running"} {
		if !strings.Contains(out, want) {
			t.Errorf("the output is missing %q:\n%s", want, out)
		}
	}
}

// The old table spelled "Not Downloaded" into the size column, repeating what
// the version mark already says. Blank columns all read "-" now.
func TestDisplayServiceTableUsesDashForEmptySize(t *testing.T) {
	out := captureStdout(t, func() {
		displayServiceTable([]HumanServiceInfo{
			{Service: "cm-ant", Version: "0.5.4 ✗", Status: "Not Found",
				Healthy: "-", InternalPort: "-", ExternalPort: "-", ImageSize: "-"},
		})
	})

	if strings.Contains(out, "Not Downloaded") {
		t.Errorf("the table still says \"Not Downloaded\":\n%s", out)
	}
}

// tableLines keeps only the lines that make up the table itself.
func tableLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "│") || strings.HasPrefix(line, "┌") ||
			strings.HasPrefix(line, "├") || strings.HasPrefix(line, "└") {
			lines = append(lines, line)
		}
	}
	return lines
}

// headingsOf lists the category headings the rows introduce, in order.
func headingsOf(rows []tableRow) []string {
	var headings []string
	for _, row := range rows {
		if row.heading != "" {
			headings = append(headings, row.heading)
		}
	}
	return headings
}

// servicesOf lists the service names the rows carry, in order, skipping rules
// and the blank service column of a follow-up row.
func servicesOf(rows []tableRow) []string {
	var names []string
	for _, row := range rows {
		if row.separator || row.heading != "" || row.cells[0] == "" {
			continue
		}
		names = append(names, row.cells[0])
	}
	return names
}

// Services are grouped under a heading per category, and the categories come out
// in the fixed display order rather than the order the services were passed in.
func TestBuildGroupedTableRowsOrdersCategories(t *testing.T) {
	rows := buildGroupedTableRows([]HumanServiceInfo{
		matching("ant-postgres"),
		matching("cm-butterfly-api"),
		matching("cb-spider"),
		matching("openbao"),
		matching("cm-ant"),
	})

	want := []string{
		"🎯 " + CategoryCoreInfra,
		"🧩 " + CategoryFrameworks,
		"🖥️ " + CategoryWebConsole,
		"🔐 " + CategorySecrets,
		"🗄️ " + CategoryDataStores,
	}
	got := headingsOf(rows)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("headings = %q, want %q", got, want)
	}
}

// A category nothing landed in prints no heading; the table shows the groups it
// actually has and nothing else.
func TestBuildGroupedTableRowsSkipsEmptyCategories(t *testing.T) {
	rows := buildGroupedTableRows([]HumanServiceInfo{matching("cb-spider")})

	want := []string{"🎯 " + CategoryCoreInfra}
	if got := headingsOf(rows); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("headings = %q, want %q", got, want)
	}
	for _, unwanted := range []string{CategoryObjectStorage, CategoryDependencies, CategorySecrets} {
		for _, heading := range headingsOf(rows) {
			if strings.Contains(heading, unwanted) {
				t.Errorf("the table heads an empty category %q", unwanted)
			}
		}
	}
}

// Services arrive from a map, so the order they are handed over in is not
// stable. Inside a category they are sorted by name, which makes the listing the
// same on every run.
func TestBuildGroupedTableRowsSortsWithinCategory(t *testing.T) {
	rows := buildGroupedTableRows([]HumanServiceInfo{
		matching("mc-terrarium"),
		matching("cb-tumblebug"),
		matching("cb-mapui"),
		matching("cb-spider"),
	})

	want := []string{"cb-mapui", "cb-spider", "cb-tumblebug", "mc-terrarium"}
	if got := servicesOf(rows); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("services = %q, want %q", got, want)
	}
}

// The same input renders identically however it is ordered on the way in, which
// is what "the table does not shuffle between runs" means in practice.
func TestDisplayServiceTableIsDeterministic(t *testing.T) {
	forward := []HumanServiceInfo{
		matching("cb-spider"), matching("cm-ant"), matching("openbao"),
		mismatched("cb-tumblebug", "0.12.02"), matching("ant-postgres"),
	}
	reversed := make([]HumanServiceInfo, 0, len(forward))
	for i := len(forward) - 1; i >= 0; i-- {
		reversed = append(reversed, forward[i])
	}

	first := captureStdout(t, func() { displayServiceTable(forward) })
	second := captureStdout(t, func() { displayServiceTable(forward) })
	third := captureStdout(t, func() { displayServiceTable(reversed) })

	if first != second {
		t.Errorf("two renders of the same input differ:\n%s\n---\n%s", first, second)
	}
	if first != third {
		t.Errorf("the render depends on the input order:\n%s\n---\n%s", first, third)
	}
}

// A heading is fenced above and below, and never leaves two rules touching —
// including where a version mismatch draws its own rule right beside one.
func TestBuildGroupedTableRowsRulesDoNotDouble(t *testing.T) {
	rows := buildGroupedTableRows([]HumanServiceInfo{
		mismatched("cb-spider", "0.9.9"),
		mismatched("cm-ant", "0.5.3"),
		matching("openbao"),
	})

	for i := 1; i < len(rows); i++ {
		if rows[i].separator && rows[i-1].separator {
			t.Errorf("rows %d and %d are both rules", i-1, i)
		}
	}
	if rows[0].heading == "" {
		t.Error("the table does not open with a heading")
	}
	if last := rows[len(rows)-1]; last.separator {
		t.Error("the table ends with a rule, want the bottom border to close it")
	}
	// Every heading is followed by a rule that separates it from its services.
	for i, row := range rows {
		if row.heading == "" {
			continue
		}
		if i+1 >= len(rows) || !rows[i+1].separator {
			t.Errorf("the heading %q is not ruled off from its services", row.heading)
		}
	}
}

// Headings span the table instead of sitting in the SERVICE column, so adding
// them leaves every column exactly where it was.
func TestHumanTableWidthsUnchangedByHeadings(t *testing.T) {
	services := []HumanServiceInfo{
		matching("cb-spider"), matching("cm-ant"), matching("cm-grasshopper-rustfs"),
		mismatched("cb-tumblebug", "0.12.02"),
	}

	plain := humanTableWidths(buildTableRows(services))
	grouped := humanTableWidths(buildGroupedTableRows(services))

	if plain != grouped {
		t.Errorf("column widths changed with headings: %v, want %v", grouped, plain)
	}
}

// A category name wider than the table is not dropped and does not push the
// columns around: the last column absorbs the difference, so the line still ends
// where the others do.
func TestHumanTableWidthsFitAnOversizedHeading(t *testing.T) {
	rows := []tableRow{
		{heading: strings.Repeat("x", 400)},
		{cells: [humanTableColumns]string{"cb-spider", "1.0.0 ✓", "running", "✓", "8080", "8080", "10MB"}},
	}

	widths := humanTableWidths(rows)
	if innerWidth(widths) < 400 {
		t.Errorf("inner width = %d, want at least 400", innerWidth(widths))
	}
}

// Every printed line stays the same width in terminal columns once headings are
// in the table, rules and follow-up rows included.
func TestDisplayServiceTableGroupedLinesAlign(t *testing.T) {
	out := captureStdout(t, func() {
		displayServiceTable([]HumanServiceInfo{
			matching("cb-spider"),
			mismatched("cb-tumblebug", "0.12.02"),
			matching("cm-ant"),
			matching("cm-butterfly-api"),
			matching("cm-cicada"),
			matching("openbao"),
			matching("ant-postgres"),
			matching("cm-grasshopper-rustfs"),
		})
	})

	lines := tableLines(out)
	if len(lines) == 0 {
		t.Fatal("no table lines were printed")
	}
	width := getDisplayWidth(lines[0])
	for _, line := range lines {
		if got := getDisplayWidth(line); got != width {
			t.Errorf("line %q is %d columns wide, want %d", line, got, width)
		}
	}

	// All eight categories are headed, each exactly once.
	for _, category := range []string{
		CategoryCoreInfra, CategoryFrameworks, CategoryWebConsole, CategoryWorkflow,
		CategorySecrets, CategoryDataStores, CategoryObjectStorage,
	} {
		if n := strings.Count(out, category); n != 1 {
			t.Errorf("category %q appears %d times, want 1", category, n)
		}
	}
}

// With -s the table only holds the services asked for, so only their categories
// are headed — a lone request does not print the whole category list.
func TestDisplayServiceTableWithDependenciesHeadsOnlyItsGroups(t *testing.T) {
	out := captureStdout(t, func() {
		displayServiceTableWithDependencies([]HumanServiceInfo{
			matching("cm-ant"),
			matching("ant-postgres"),
		}, []string{"cm-ant"})
	})

	for _, want := range []string{"🎯 Requested Services:", CategoryFrameworks, CategoryDataStores} {
		if !strings.Contains(out, want) {
			t.Errorf("the output is missing %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{CategoryCoreInfra, CategoryWebConsole, CategorySecrets, CategoryObjectStorage} {
		if strings.Contains(out, unwanted) {
			t.Errorf("the output heads %q, which nothing was requested from:\n%s", unwanted, out)
		}
	}

	// The two tables are sized independently, but each one lines up with itself.
	lines := tableLines(out)
	if len(lines) == 0 {
		t.Fatal("no table lines were printed")
	}
	width := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "┌") {
			width = getDisplayWidth(line)
		}
		if got := getDisplayWidth(line); got != width {
			t.Errorf("line %q is %d columns wide, want %d", line, got, width)
		}
	}
}

// U+FE0F asks for the emoji form of the character before it, which terminals
// draw double-width. Counting the selector as nothing left "⚙️" measuring one
// column and the heading it opens one short of the border.
func TestGetDisplayWidthCountsEmojiPresentation(t *testing.T) {
	cases := map[string]int{
		"⚙️":  2, // U+2699 + U+FE0F: narrow base, promoted
		"🖥️":  2, // U+1F5A5 + U+FE0F: already wide, unchanged
		"🎯":   2,
		"✓":   1,
		"abc": 3,
	}
	for input, want := range cases {
		if got := getDisplayWidth(input); got != want {
			t.Errorf("getDisplayWidth(%q) = %d, want %d", input, got, want)
		}
	}
}

// Every icon the headings use is measured as two columns, so no category can
// knock the heading row out of line with the rest of the table.
func TestCategoryIconsMeasureTwoColumns(t *testing.T) {
	for _, entry := range categoryDisplayOrder {
		if got := getDisplayWidth(entry.Icon); got != 2 {
			t.Errorf("icon %q of %q measures %d columns, want 2", entry.Icon, entry.Name, got)
		}
	}
	if got := getDisplayWidth(unknownCategoryIcon); got != 2 {
		t.Errorf("the fallback icon measures %d columns, want 2", got)
	}
}
