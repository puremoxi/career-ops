package model

// CareerApplication represents a single job application from the tracker.
type CareerApplication struct {
	Number       int
	Date         string
	Company      string
	Role         string
	Status       string
	Score        float64
	ScoreRaw     string
	HasPDF       bool
	ReportPath   string
	ReportNumber string
	Notes        string
	JobURL       string // URL of the original job posting
	// Derived from Notes free-text (see data.deriveNoteFields)
	Location    string  // "City, ST" when a US city+state appears in the notes
	WorkMode    string  // "Remote" | "Hybrid" | "Full" (onsite), "" when unknown
	PayRange    string  // first $-range found in the notes, e.g. "$140-210K"
	PayMax      float64 // top of PayRange in dollars (sort key), 0 when unknown
	PaySource   string  // "POSTED" when the JD listed it, "est" for estimates, "" unknown
	LastContact string  // max YYYY-MM-DD found in notes (falls back to applied date)
	// Enrichment (lazy loaded from report)
	Archetype    string
	TlDr         string
	Remote       string
	CompEstimate string
}

// PipelineMetrics holds aggregate stats for the pipeline dashboard.
type PipelineMetrics struct {
	Total      int
	ByStatus   map[string]int
	AvgScore   float64
	TopScore   float64
	WithPDF    int
	Actionable int
}

// ProgressMetrics holds job search progress analytics.
type ProgressMetrics struct {
	// Funnel
	FunnelStages []FunnelStage

	// Score distribution
	ScoreBuckets []ScoreBucket

	// Timeline (weekly activity)
	WeeklyActivity []WeekActivity

	// Rates
	ResponseRate  float64 // Responded / Applied
	InterviewRate float64 // Interview / Applied
	OfferRate     float64 // Offer / Applied

	// Averages
	AvgScore    float64
	TopScore    float64
	TotalOffers int
	ActiveApps  int // not skip/rejected/discarded
}

// FunnelStage represents one stage of the application funnel.
type FunnelStage struct {
	Label string
	Count int
	Pct   float64 // percentage of total
}

// ScoreBucket represents a score range and its count.
type ScoreBucket struct {
	Label string // e.g., "4.5-5.0", "4.0-4.4", "3.5-3.9", "3.0-3.4", "<3.0"
	Count int
}

// WeekActivity represents application activity for a given ISO week.
type WeekActivity struct {
	Week  string // e.g., "2026-W14", "2026-W13"
	Count int
}

// ScanRun represents one row of data/scan-runs.tsv — the per-run counters
// appended by scan.mjs each time `node scan.mjs` completes.
type ScanRun struct {
	Timestamp  string // raw ISO 8601, e.g. "2026-07-15T18:58:12.710Z"
	Status     string // "completed" | "failed"
	Companies  int
	Boards     int
	Found      int
	Filtered   int // sum of all filtered_* columns for this row
	Dupes      int
	NewAdded   int
	Errors     int
	DurationMs int // 0/unknown when the row predates duration tracking
}

// ScanHistoryMetrics holds aggregate stats over all scan runs, mirroring
// stats.mjs's computeRunStats() so the dashboard and CLI report the same numbers.
type ScanHistoryMetrics struct {
	Runs             []ScanRun // newest first
	TotalRuns        int
	FailedRuns       int
	AvgFoundPerRun   float64
	AvgNewPerRun     float64
	AvgDurationMs    float64 // average over runs with known duration; 0 if none known
	FilterRemovalPct float64
}
