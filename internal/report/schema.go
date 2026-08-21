package report

type jsonGroup struct {
	Values   map[string]string `json:"values"`
	Count    int               `json:"count"`
	FirstRow int64             `json:"first_row"`

	ValuesEscaped bool `json:"values_escaped,omitempty"`
}

type jsonRisk struct {
	Model   string  `json:"model"`
	Highest float64 `json:"highest"`
	Average float64 `json:"average"`
	Lowest  float64 `json:"lowest"`
}

type jsonConcern struct {
	Key     string `json:"key"`
	Message string `json:"message"`
}

type jsonDataset struct {
	Name             string   `json:"name,omitempty"`
	QuasiIdentifiers []string `json:"quasi_identifiers"`
	ThresholdK       int      `json:"threshold_k"`
	K                int      `json:"k"`

	Passed        bool          `json:"passed"`
	KThresholdMet bool          `json:"k_threshold_met"`
	Concerns      []jsonConcern `json:"concerns"`

	RowsChecked    int64   `json:"rows_checked"`
	RowsSuppressed int64   `json:"rows_suppressed"`
	RowsEmptyQI    int64   `json:"rows_with_empty_quasi_identifier"`
	Coverage       float64 `json:"coverage"`

	Groups     int   `json:"groups"`
	RowsAtRisk int64 `json:"rows_at_risk"`
	UniqueRows int64 `json:"unique_rows"`

	Risk jsonRisk `json:"risk"`

	GroupsBelowThreshold int         `json:"groups_below_threshold"`
	WorstGroups          []jsonGroup `json:"worst_groups"`
	WorstGroupsTruncated bool        `json:"worst_groups_truncated"`

	LargestGroup jsonGroup `json:"largest_group"`

	Diversity []jsonDiversity `json:"l_diversity,omitempty"`
	Closeness []jsonCloseness `json:"t_closeness,omitempty"`

	Measured    []string `json:"measured"`
	NotMeasured []string `json:"not_measured"`
}

type jsonDiversity struct {
	Attribute string `json:"attribute"`
	L         int    `json:"l"`
	WorstRow  int64  `json:"worst_row"`
}

type jsonCloseness struct {
	Attribute string  `json:"attribute"`
	Kind      string  `json:"kind"`
	T         float64 `json:"t"`
	WorstRow  int64   `json:"worst_row"`
}

type Telemetry struct {
	ElapsedMS int64            `json:"elapsed_ms"`
	Spans     []Span           `json:"spans"`
	Metrics   map[string]int64 `json:"metrics"`
	Errors    []string         `json:"errors,omitempty"`
}

type Span struct {
	Name       string `json:"name"`
	DurationMS int64  `json:"duration_ms"`
	Failed     bool   `json:"failed,omitempty"`
	Children   []Span `json:"children,omitempty"`
}

type jsonReport struct {
	Datasets  []jsonDataset `json:"datasets"`
	Strict    bool          `json:"strict"`
	Failed    bool          `json:"failed"`
	Telemetry *Telemetry    `json:"telemetry,omitempty"`
}
