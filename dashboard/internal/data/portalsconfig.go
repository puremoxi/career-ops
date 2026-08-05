package data

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PortalsConfigSummary is a minimal read of portals.yml's two top-level
// scan-source lists — just enough to size the "how much of the configured
// fleet is dead weight" percentages on the scan-effectiveness screen, and to
// show the true configured totals alongside whatever a given run actually
// touched. This is a hand-rolled scan rather than a real YAML parse: go.mod
// carries no YAML dependency, and both lists share a stable, narrow shape (a
// `- name:` list item, an `enabled:` line somewhere inside it) that a full
// parser would be overkill for. If portals.yml's structure changes shape,
// this degrades to under-counting rather than crashing.
//
// Companies come from tracked_companies: one entry per single employer's
// careers page/API. BoardQueries come from search_queries: cross-cutting
// WebSearch queries against a job-board platform (Ashby, Greenhouse,
// Workable, Remotive, WeWorkRemotely, Working Nomads, a16z Speedrun Talent
// Network, etc.) that lists postings from many employers at once — these
// are the actual "job boards" this scanner uses, unrelated to the unused
// `job_boards:` YAML key scan.mjs's own (always-zero) `boards` counter reads.
type PortalsConfigSummary struct {
	TotalCompanies    int
	EnabledCompanies  int
	DisabledCompanies int

	TotalBoardQueries    int
	EnabledBoardQueries  int
	DisabledBoardQueries int
}

var (
	trackedCompaniesKeyRe = regexp.MustCompile(`^tracked_companies:\s*$`)
	searchQueriesKeyRe    = regexp.MustCompile(`^search_queries:\s*$`)
	companyNameRe         = regexp.MustCompile(`^\s*-\s*name:\s*.+$`)
	nameCaptureRe         = regexp.MustCompile(`^\s*-?\s*name:\s*(.+?)\s*$`)
	queryCaptureRe        = regexp.MustCompile(`^\s*(?:scan_)?query:\s*(.+?)\s*$`)
	enabledLineRe         = regexp.MustCompile(`^\s*enabled:\s*(true|false)\s*$`)
	topLevelKeyRe         = regexp.MustCompile(`^\S`) // a line starting at column 0 (not indented, not blank)
)

// listBlockCounts counts `- name:` entries and their enabled/disabled split
// within one top-level YAML list block, identified by blockKeyRe matching
// its opening `key:` line. Shared by tracked_companies and search_queries —
// both use the identical list-item shape.
func listBlockCounts(lines []string, blockKeyRe *regexp.Regexp) (total, enabled, disabled int) {
	inBlock := false
	haveEntry := false
	entryEnabled := true // enabled: true is the default when the field is omitted

	finalizeEntry := func() {
		if !haveEntry {
			return
		}
		total++
		if entryEnabled {
			enabled++
		} else {
			disabled++
		}
	}

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		if !inBlock {
			if blockKeyRe.MatchString(line) {
				inBlock = true
				haveEntry = false
			}
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// A non-indented, non-comment line ends this block (the next
		// top-level YAML key).
		if topLevelKeyRe.MatchString(line) {
			finalizeEntry()
			inBlock = false
			haveEntry = false
			continue
		}

		if companyNameRe.MatchString(line) {
			finalizeEntry()
			haveEntry = true
			entryEnabled = true
			continue
		}

		if m := enabledLineRe.FindStringSubmatch(line); m != nil {
			entryEnabled = m[1] == "true"
		}
	}
	if inBlock {
		finalizeEntry()
	}

	return total, enabled, disabled
}

// ParsePortalsConfig reads portals.yml at the career-ops root (not under
// data/ — portals.yml lives at the project root per AGENTS.md). A missing
// file is not an error and returns a zero-value summary.
func ParsePortalsConfig(careerOpsPath string) PortalsConfigSummary {
	raw, err := os.ReadFile(filepath.Join(careerOpsPath, "portals.yml"))
	if err != nil {
		return PortalsConfigSummary{}
	}

	lines := strings.Split(string(raw), "\n")

	var s PortalsConfigSummary
	s.TotalCompanies, s.EnabledCompanies, s.DisabledCompanies = listBlockCounts(lines, trackedCompaniesKeyRe)
	s.TotalBoardQueries, s.EnabledBoardQueries, s.DisabledBoardQueries = listBlockCounts(lines, searchQueriesKeyRe)

	return s
}

// ParseQueryProfiles reads portals.yml's `search_queries` (name -> query)
// and `tracked_companies` (name -> scan_query, when present) into a single
// name -> search-terms map. Used by the scan-effectiveness screen to show
// what titles/keywords a given portal row in scan-history.tsv was actually
// searching for. Same hand-rolled-scan caveat as ParsePortalsConfig: no YAML
// dependency, so a structural change to portals.yml degrades to missing
// entries rather than a crash.
func ParseQueryProfiles(careerOpsPath string) map[string]string {
	raw, err := os.ReadFile(filepath.Join(careerOpsPath, "portals.yml"))
	if err != nil {
		return map[string]string{}
	}

	lines := strings.Split(string(raw), "\n")
	result := make(map[string]string)

	inSection := false
	var curName, curQuery string

	flush := func() {
		if curName != "" && curQuery != "" {
			if _, exists := result[curName]; !exists {
				result[curName] = curQuery
			}
		}
		curName, curQuery = "", ""
	}

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		if !inSection {
			if trackedCompaniesKeyRe.MatchString(line) || searchQueriesKeyRe.MatchString(line) {
				inSection = true
			}
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if topLevelKeyRe.MatchString(line) {
			flush()
			inSection = trackedCompaniesKeyRe.MatchString(line) || searchQueriesKeyRe.MatchString(line)
			continue
		}

		if companyNameRe.MatchString(line) {
			flush()
			if m := nameCaptureRe.FindStringSubmatch(line); m != nil {
				curName = cleanYAMLScalar(m[1])
			}
			continue
		}

		if curQuery == "" {
			if m := queryCaptureRe.FindStringSubmatch(line); m != nil {
				curQuery = cleanYAMLScalar(m[1])
			}
		}
	}
	flush()

	return result
}

// cleanYAMLScalar strips a wrapping pair of single or double quotes, the
// only quoting style portals.yml uses for these fields.
func cleanYAMLScalar(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
