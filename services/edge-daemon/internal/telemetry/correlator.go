package telemetry

import (
	"math"
	"sync"
	"time"
)

type ProcessMeasurement struct {
	StationID             string        `json:"station_id"`
	OperatorID            string        `json:"operator_id"`
	BundleID              string        `json:"bundle_id"`
	CycleDuration         time.Duration `json:"cycle_duration"`
	InterprocessLatency   time.Duration `json:"interprocess_latency"`
	FactorySAM            time.Duration `json:"factory_sam"`
	PaceDeviationPercent  float64       `json:"pace_deviation_percent"`
	IsBottleneck          bool          `json:"is_bottleneck"`
}

type ProcessCorrelator struct {
	mu           sync.Mutex
	lastSeenTime map[string]time.Time // BundleID -> Timestamp of completion at Station N-1
	cycleStarts  map[string]time.Time // StationID -> Timestamp of cycle start
}

func NewProcessCorrelator() *ProcessCorrelator {
	return &ProcessCorrelator{
		lastSeenTime: make(map[string]time.Time),
		cycleStarts:  make(map[string]time.Time),
	}
}

func (c *ProcessCorrelator) RecordStationEntry(stationID, bundleID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cycleStarts[stationID] = time.Now().UTC()
}

func (c *ProcessCorrelator) RecordCycleCompletion(stationID, operatorID, bundleID string, samSeconds float64) ProcessMeasurement {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UTC()
	start, exists := c.cycleStarts[stationID]
	cycleDuration := 36 * time.Second
	if exists {
		cycleDuration = now.Sub(start)
	}

	var interLatency time.Duration
	if prevTime, ok := c.lastSeenTime[bundleID]; ok {
		interLatency = now.Sub(prevTime)
	}
	c.lastSeenTime[bundleID] = now

	factorySAM := time.Duration(samSeconds * float64(time.Second))
	paceDev := 0.0
	if factorySAM > 0 {
		paceDev = (cycleDuration.Seconds() - factorySAM.Seconds()) / factorySAM.Seconds() * 100.0
	}

	isBottleneck := paceDev > 15.0 || interLatency > 15*time.Minute

	return ProcessMeasurement{
		StationID:            stationID,
		OperatorID:           operatorID,
		BundleID:             bundleID,
		CycleDuration:        cycleDuration,
		InterprocessLatency:  interLatency,
		FactorySAM:           factorySAM,
		PaceDeviationPercent: math.Round(paceDev*10) / 10,
		IsBottleneck:         isBottleneck,
	}
}