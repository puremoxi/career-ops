package data

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// salaryObservation is one row from data/salary-observations.tsv, the
// append-only structured comp log written by career-ops's oferta/auto-pipeline
// modes (advertised, from the JD) and by the user reporting a confirmed figure
// (actual, from a recruiter/offer letter/contract).
type salaryObservation struct {
	kind     string // "advertised" | "actual"
	amount   string
	currency string
	date     string
}

// String renders the observation the same way the dashboard's Notes-derived
// PayRange is rendered ("$280,000-$420,000 CAD"), so both sources look
// identical in the Pay column regardless of which one supplied the value.
func (o salaryObservation) String() string {
	if o.currency == "" {
		return o.amount
	}
	return o.amount + " " + o.currency
}

// loadSalaryObservations reads data/salary-observations.tsv (optional; a
// missing file is not an error, same convention as LoadPDFManifest) into a
// tracker# → best-observation map.
//
// WHY this exists: the dashboard's Pay column used to come from two sources
// only — a regex over the tracker's free-text Notes, and a report-cache
// fallback that's populated lazily (only after the user opens that specific
// report in the dashboard session). Both can miss a figure the evaluation
// already captured structurally in salary-observations.tsv (report Block D's
// advertised_comp, or a user-confirmed actual figure) if the Notes summary
// sentence happened not to restate the number in $-parseable form. This is
// a third, eager, structured source consulted before either of those.
//
// "Best" prefers a confirmed `actual` figure over the JD's `advertised`
// range — a `desired` line is the candidate's own ask, not what the job
// pays, and is never used here. Among same-kind rows for the same tracker#,
// the most recently dated line wins (the log is append-only, so a later
// renegotiation is a new line, never an edit of a prior one).
func loadSalaryObservations(careerOpsPath string) map[int]salaryObservation {
	result := make(map[int]salaryObservation)
	raw, err := os.ReadFile(filepath.Join(careerOpsPath, "data", "salary-observations.tsv"))
	if err != nil {
		return result
	}
	rank := map[string]int{"actual": 2, "advertised": 1}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		num, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			continue
		}
		kind := strings.TrimSpace(fields[2])
		if _, tracked := rank[kind]; !tracked {
			continue // "desired" (candidate's own ask) is not the job's pay
		}
		obs := salaryObservation{
			kind:     kind,
			amount:   strings.TrimSpace(fields[3]),
			currency: strings.TrimSpace(fields[4]),
			date:     strings.TrimSpace(fields[1]),
		}
		if obs.amount == "" {
			continue
		}
		existing, ok := result[num]
		if !ok || rank[kind] > rank[existing.kind] || (rank[kind] == rank[existing.kind] && obs.date >= existing.date) {
			result[num] = obs
		}
	}
	return result
}
