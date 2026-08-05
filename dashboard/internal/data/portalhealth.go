package data

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// portalHealthDeadStreak mirrors stats.mjs's computePortalStats default
// threshold (cfg.portal_health_threshold ?? 3). The dashboard doesn't parse
// portals.yml for an override — 3 is the documented default everywhere else.
const portalHealthDeadStreak = 3

// PortalHealthRecord is one row from data/portal-health.tsv, written by
// scan.mjs (via verify-portals.mjs's classifyFetchError) every run for every
// company it can reach directly (API/local-parser — the zero-token tier).
type PortalHealthRecord struct {
	Timestamp string
	Company   string
	Status    string // reachable, empty, network, slug_gone, auth, server, unknown
}

// PortalHealthRun groups records from the same scan run (timestamps are
// truncated to the minute, since scan.mjs and the health-check pass can log
// sub-second-apart timestamps for what is operationally one run).
type PortalHealthRun struct {
	RunKey    string // minute-truncated timestamp, used to correlate with ScanRun.Timestamp
	Timestamp string // representative (first-seen) full timestamp for display
	Companies int    // distinct companies checked directly this run — the zero-token coverage
	Reachable int
	Empty     int
	Errored   int // network + slug_gone + auth + server + unknown
}

// PortalHealthMetrics is the aggregated view ParsePortalHealth returns.
type PortalHealthMetrics struct {
	Runs                  []PortalHealthRun // newest first
	LatestReachable       int
	LatestEmpty           int
	LatestErrored         int
	LatestTotal           int
	PersistentlyDead      []string // company names currently on a >= portalHealthDeadStreak failure streak, sorted
	PersistentlyDeadCount int
}

// runKey truncates an ISO 8601 timestamp to minute precision.
func runKey(ts string) string {
	if len(ts) >= 16 {
		return ts[:16]
	}
	return ts
}

// ParsePortalHealth reads data/portal-health.tsv under careerOpsPath. A
// missing file is not an error (same convention as ParseScanRuns) and
// returns a zero-value PortalHealthMetrics.
func ParsePortalHealth(careerOpsPath string) PortalHealthMetrics {
	raw, err := os.ReadFile(filepath.Join(careerOpsPath, "data", "portal-health.tsv"))
	if err != nil {
		return PortalHealthMetrics{}
	}

	lines := strings.Split(string(raw), "\n")
	if len(lines) <= 1 {
		return PortalHealthMetrics{}
	}

	var records []PortalHealthRecord
	for _, line := range lines[1:] {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue // torn row
		}
		records = append(records, PortalHealthRecord{
			Timestamp: strings.TrimSpace(fields[0]),
			Company:   strings.TrimSpace(fields[1]),
			Status:    strings.TrimSpace(fields[2]),
		})
	}
	if len(records) == 0 {
		return PortalHealthMetrics{}
	}

	// Chronological order for the per-company streak walk below.
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Timestamp < records[j].Timestamp
	})

	runsByKey := make(map[string]*PortalHealthRun)
	var runOrder []string
	streaks := make(map[string]int)

	for _, r := range records {
		key := runKey(r.Timestamp)
		run, ok := runsByKey[key]
		if !ok {
			run = &PortalHealthRun{RunKey: key, Timestamp: r.Timestamp}
			runsByKey[key] = run
			runOrder = append(runOrder, key)
		}
		run.Companies++
		switch r.Status {
		case "reachable":
			run.Reachable++
			streaks[r.Company] = 0
		case "empty":
			run.Empty++
			streaks[r.Company] = 0
		default:
			run.Errored++
			streaks[r.Company]++
		}
	}

	var runs []PortalHealthRun
	for _, key := range runOrder {
		runs = append(runs, *runsByKey[key])
	}
	sort.SliceStable(runs, func(i, j int) bool {
		return runs[i].RunKey > runs[j].RunKey // newest first
	})

	metrics := PortalHealthMetrics{Runs: runs}
	if len(runs) > 0 {
		latest := runs[0]
		metrics.LatestReachable = latest.Reachable
		metrics.LatestEmpty = latest.Empty
		metrics.LatestErrored = latest.Errored
		metrics.LatestTotal = latest.Companies
	}

	var dead []string
	for company, streak := range streaks {
		if streak >= portalHealthDeadStreak {
			dead = append(dead, company)
		}
	}
	sort.Strings(dead)
	metrics.PersistentlyDead = dead
	metrics.PersistentlyDeadCount = len(dead)

	return metrics
}
