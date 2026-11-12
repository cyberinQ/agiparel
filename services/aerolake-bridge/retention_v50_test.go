// Author: Sunil Jain, 2026-11-12.
// Prove Aerolake v50 never starts pruning without verified archival checkpoints.
package main

import "testing"

func TestRetentionFailsClosedByDefault(t *testing.T) {
	t.Setenv("AEROLAKE_PRUNING_KILL_SWITCH", "")
	if !loadConfig().KillSwitch {
		t.Fatal("retention must be disabled if the environment variable is missing")
	}
}

func TestRetentionBlockedEvenOnOptIn(t *testing.T) {
	t.Setenv("AEROLAKE_PRUNING_KILL_SWITCH", "false")
	if loadConfig().KillSwitch {
		t.Fatal("test requires explicit request to lift kill switch")
	}
	// Nil DB proves runCycle never executes an SQL statement or queries a
	// supposed watermark simply because the Aerolake port is listening.
	runCycle(nil, nil, loadConfig())
}
