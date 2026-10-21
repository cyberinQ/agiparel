package telemetry

import (
	"errors"
	"math"
	"sync"
	"time"
)

type ProcessMeasurement struct {
	StationID            string        `json:"station_id"`
	OperatorID           string        `json:"operator_id"`
	BundleID             string        `json:"bundle_id"`
	CycleDuration        time.Duration `json:"cycle_duration"`
	InterprocessLatency  time.Duration `json:"interprocess_latency"`
	FactorySAM           time.Duration `json:"factory_sam"`
	PaceDeviationPercent float64       `json:"pace_deviation_percent"`
	IsBottleneck         bool          `json:"is_bottleneck"`
}

type ProcessCorrelator struct {
	mu           sync.Mutex
	lastSeenTime map[string]time.Time
	cycleStarts  map[string]time.Time
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
	key := stationID + ":" + bundleID
	c.cycleStarts[key] = time.Now().UTC()
}

func (c *ProcessCorrelator) RecordCycleCompletion(stationID, operatorID, bundleID string, samSeconds float64) (ProcessMeasurement, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UTC()
	key := stationID + ":" + bundleID
	start, exists := c.cycleStarts[key]
	if !exists {
		return ProcessMeasurement{}, errors.New("ERR_MISSING_ENTRY_SCAN: Bundle entry scan not recorded at station")
	}
	cycleDuration := now.Sub(start)
	delete(c.cycleStarts, key)

	var interLatency time.Duration
	if prevTime, ok := c.lastSeenTime[bundleID]; ok {
		interLatency = start.Sub(prevTime)
		if interLatency < 0 {
			interLatency = 0
		}
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
	}, nil
}
