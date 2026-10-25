// Author: Sunil Jain, 2026-10-25
// Verify that document facts retain unknown fields without altering them.
package documentcapture

import (
	"encoding/json"
	"testing"
)

func TestFlattenUnknownFields(t *testing.T) {
	input := map[string]any{"buyer": map[string]any{"po": "ABC", "extra": []any{12.5, "cotton"}}, "missing": nil}
	facts, err := Flatten(input)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range facts {
		got[f.Path] = string(f.Value)
	}
	if got["/buyer/po"] != `"ABC"` || got["/buyer/extra/0"] != "12.5" || got["/buyer/extra/1"] != `"cotton"` {
		t.Fatalf("lost raw facts: %+v", got)
	}
	if _, ok := got["/missing"]; ok {
		t.Fatal("invented missing fact")
	}
	_, err = json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
}
