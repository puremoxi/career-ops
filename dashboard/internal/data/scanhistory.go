package data

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScanHistoryEntry is one row from data/scan-history.tsv. Only the columns
// the effectiveness screen needs are kept; jd_fingerprint, postedAt, and any
// trailing work-model columns (see modes/scan.md's "Work-model tagging")
// are read defensively but not surfaced here.
type ScanHistoryEntry struct {
	URL       string
	FirstSeen string
	Portal    string
	Title     string
	Company   string
	Status    string
	Location  string
}

// PortalYield is how many rows a given portal/query name has contributed,
// and how many of those were actually added to the pipeline.
type PortalYield struct {
	Name  string
	Added int
	Total int
}

// CompanyYield is how many rows a given company has contributed that were
// actually added to the pipeline — i.e. which tracked companies are worth
// keeping vs. dead weight in portals.yml.
type CompanyYield struct {
	Name  string
	Added int
}

// ScanHistoryMetrics is the aggregated view ParseScanHistory returns.
type ScanHistoryMetrics struct {
	TotalRows           int
	AddedCount          int
	SkippedDupCount     int
	SkippedTitleCount   int
	SkippedExpiredCount int
	OtherCount          int
	CompaniesSeen       int // distinct companies across all rows
	CompaniesMatched    int // distinct companies with at least one "added" row
	EarliestFirstSeen   string
	LatestFirstSeen     string
	TopPortals          []PortalYield  // sorted by Added desc, capped
	TopCompanies        []CompanyYield // sorted by Added desc, capped

	// Percentages, all relative to TotalRows (0 when TotalRows is 0).
	AddedPct          float64
	SkippedDupPct     float64
	SkippedTitlePct   float64
	SkippedExpiredPct float64
}

// CompaniesZeroYield is how many distinct companies were scanned at least
// once but never produced an "added" row — candidates for pruning from
// portals.yml, or simply companies with no matching openings yet.
func (m ScanHistoryMetrics) CompaniesZeroYield() int {
	return m.CompaniesSeen - m.CompaniesMatched
}

// topN caps how many portal/company rows the screen renders — enough to be
// useful without needing its own scroll region.
const topN = 10

// ParseScanHistory reads data/scan-history.tsv under careerOpsPath. A missing
// file is not an error (same convention as ParseScanRuns) and returns a
// zero-value ScanHistoryMetrics. Rows are split positionally rather than by
// header name: the file's schema has grown trailing columns over time
// (jd_fingerprint, postedAt, work_model, work_model_source) without every
// historical row being backfilled, so only the first 6 required + 1 optional
// (location) columns are relied on here.
func ParseScanHistory(careerOpsPath string) ScanHistoryMetrics {
	raw, err := os.ReadFile(filepath.Join(careerOpsPath, "data", "scan-history.tsv"))
	if err != nil {
		return ScanHistoryMetrics{}
	}

	lines := strings.Split(string(raw), "\n")
	if len(lines) <= 1 {
		return ScanHistoryMetrics{}
	}

	var entries []ScanHistoryEntry
	for _, line := range lines[1:] {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		// Torn/malformed row: fewer than the 6 required columns
		// (url, first_seen, portal, title, company, status). Skip it.
		if len(fields) < 6 {
			continue
		}
		entry := ScanHistoryEntry{
			URL:       strings.TrimSpace(fields[0]),
			FirstSeen: strings.TrimSpace(fields[1]),
			Portal:    strings.TrimSpace(fields[2]),
			Title:     strings.TrimSpace(fields[3]),
			Company:   strings.TrimSpace(fields[4]),
			Status:    strings.TrimSpace(fields[5]),
		}
		if len(fields) > 6 {
			entry.Location = strings.TrimSpace(fields[6])
		}
		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return ScanHistoryMetrics{}
	}

	metrics := ScanHistoryMetrics{TotalRows: len(entries)}

	companiesSeen := make(map[string]bool)
	companiesMatched := make(map[string]bool)
	portalTotals := make(map[string]*PortalYield)
	companyTotals := make(map[string]*CompanyYield)

	for _, e := range entries {
		switch e.Status {
		case "added":
			metrics.AddedCount++
		case "skipped_dup":
			metrics.SkippedDupCount++
		case "skipped_title":
			metrics.SkippedTitleCount++
		case "skipped_expired":
			metrics.SkippedExpiredCount++
		default:
			metrics.OtherCount++
		}

		if e.FirstSeen != "" {
			if metrics.EarliestFirstSeen == "" || e.FirstSeen < metrics.EarliestFirstSeen {
				metrics.EarliestFirstSeen = e.FirstSeen
			}
			if e.FirstSeen > metrics.LatestFirstSeen {
				metrics.LatestFirstSeen = e.FirstSeen
			}
		}

		if e.Company != "" {
			companiesSeen[e.Company] = true
			if _, ok := companyTotals[e.Company]; !ok {
				companyTotals[e.Company] = &CompanyYield{Name: e.Company}
			}
			if e.Status == "added" {
				companiesMatched[e.Company] = true
				companyTotals[e.Company].Added++
			}
		}

		if e.Portal != "" {
			if _, ok := portalTotals[e.Portal]; !ok {
				portalTotals[e.Portal] = &PortalYield{Name: e.Portal}
			}
			portalTotals[e.Portal].Total++
			if e.Status == "added" {
				portalTotals[e.Portal].Added++
			}
		}
	}

	metrics.CompaniesSeen = len(companiesSeen)
	metrics.CompaniesMatched = len(companiesMatched)

	metrics.TopPortals = topPortals(portalTotals)
	metrics.TopCompanies = topCompanies(companyTotals)

	if metrics.TotalRows > 0 {
		total := float64(metrics.TotalRows)
		metrics.AddedPct = float64(metrics.AddedCount) / total * 100
		metrics.SkippedDupPct = float64(metrics.SkippedDupCount) / total * 100
		metrics.SkippedTitlePct = float64(metrics.SkippedTitleCount) / total * 100
		metrics.SkippedExpiredPct = float64(metrics.SkippedExpiredCount) / total * 100
	}

	return metrics
}

func topPortals(m map[string]*PortalYield) []PortalYield {
	out := make([]PortalYield, 0, len(m))
	for _, v := range m {
		out = append(out, *v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Added != out[j].Added {
			return out[i].Added > out[j].Added
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > topN {
		out = out[:topN]
	}
	return out
}

func topCompanies(m map[string]*CompanyYield) []CompanyYield {
	out := make([]CompanyYield, 0, len(m))
	for _, v := range m {
		if v.Added == 0 {
			continue // dead weight — not useful in a "top companies" list
		}
		out = append(out, *v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Added != out[j].Added {
			return out[i].Added > out[j].Added
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > topN {
		out = out[:topN]
	}
	return out
}
