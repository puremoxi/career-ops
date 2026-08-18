package data

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// loadStatusDates reads data/status-log.tsv (the set-status.mjs transition
// ledger: {tracker#}\t{date}\t{from}\t{to}\t{source}\t{note}, optional and
// append-only) into a tracker# -> normalized-status -> latest-date map.
//
// WHY this exists: the tracker's own Date column is written once, at
// evaluation time, and never touched again — set-status.mjs updates Status
// and Notes only. So a row sitting in "Applied" for weeks still shows its
// original evaluation date in the dashboard's Date column, which reads as
// "when did I evaluate this" when the column header and the surrounding
// status-grouped view both imply "when did this happen for the status it's
// in now". This ledger is append-only and chronological, so the last line
// for a given (num, to) pair is that status's most recent transition date —
// including a later "correction" line, which exists precisely to supersede
// an earlier wrong date for the same transition.
func loadStatusDates(careerOpsPath string) map[int]map[string]string {
	result := make(map[int]map[string]string)
	raw, err := os.ReadFile(filepath.Join(careerOpsPath, "data", "status-log.tsv"))
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}
		num, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			continue
		}
		date := strings.TrimSpace(fields[1])
		to := NormalizeStatus(strings.TrimSpace(fields[3]))
		if date == "" || to == "" {
			continue
		}
		if result[num] == nil {
			result[num] = make(map[string]string)
		}
		result[num][to] = date // last line for this (num, to) wins — file is chronological
	}
	return result
}
