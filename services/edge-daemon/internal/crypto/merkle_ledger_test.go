package crypto

import (
	"fmt"
	"testing"
)

func TestMerkleLedger_IntegrityAndTamperDetection(t *testing.T) {
	secretKey := []byte("PILOT_LINE14_SECRET_HMAC_KEY_2026")
	ledger := NewMerkleLedger(secretKey)

	// 1. Verify genesis block
	valid, err := ledger.VerifyChain()
	if !valid || err != nil {
		t.Fatalf("expected valid genesis chain, got err: %v", err)
	}

	// 2. Append sequence of 10 events
	for i := 1; i <= 10; i++ {
		payload := fmt.Sprintf(`{"event_id":%d,"station":"SEW-14-04","action":"CYCLE_DONE"}`, i)
		block, err := ledger.AppendEvent("LINE14_ST04", "WORKER_A814", "CYCLE_DONE", payload)
		if err != nil {
			t.Fatalf("failed to append block %d: %v", i, err)
		}
		if block.Index != uint64(i) {
			t.Fatalf("expected block index %d, got %d", i, block.Index)
		}
	}

	// 3. Verify valid chain of 11 blocks (genesis + 10)
	valid, err = ledger.VerifyChain()
	if !valid || err != nil {
		t.Fatalf("chain failed integrity verification: %v", err)
	}

	// 4. Tamper injection: simulate PostgreSQL / ledger bit-flip attack
	ledger.Lock()
	ledger.chain[5].PayloadHash = "0000000000000000000000000000000000000000000000000000000000000000"
	ledger.Unlock()

	valid, err = ledger.VerifyChain()
	if valid || err == nil {
		t.Fatalf("expected verification failure on tampered ledger, but chain passed")
	}
}