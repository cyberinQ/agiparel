package interviewer

type MachineSpec struct {
	MachineID            string `json:"machine_id"`
	LineID               string `json:"line_id"`
	Name                 string `json:"name"`
	CanonicalClass       string `json:"canonical_class"`
	Function             string `json:"function"`
	CapacityUnitsPerHour int    `json:"capacity_units_per_hour"`
}

type LineSpec struct {
	LineID               string  `json:"line_id"`
	DisplayName          string  `json:"display_name"`
	ActiveStationsCount  int     `json:"active_stations_count"`
	TargetDailyCapacity  int     `json:"target_daily_capacity"`
	DefaultCycleFloorPct float64 `json:"default_cycle_floor_pct"`
}

type CandidateConfig struct {
	SchemaVersion string        `json:"schema_version"`
	TenantID      string        `json:"tenant_id"`
	FactoryName   string        `json:"factory_name"`
	UnitNumber    string        `json:"unit_number"`
	Lines         []LineSpec    `json:"lines"`
	Machines      []MachineSpec `json:"machines"`
	IsConfirmed   bool          `json:"is_confirmed"`
	ConfirmedBy   string        `json:"confirmed_by,omitempty"`
	ConfirmedAt   int64         `json:"confirmed_at,omitempty"`
}

type ExecutiveMemo struct {
	TenantID    string   `json:"tenant_id"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	ActionItems []string `json:"action_items"`
	CreatedAt   int64    `json:"created_at"`
}
