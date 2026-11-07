package flow

import "testing"

func TestStepsEqual(t *testing.T) {
	sent := `[{"id":"END","type":"END","position":{"x":800,"y":200},"data":{"components":[],"action":{"type":"EXECUTOR"}}}]`
	stored := `[{"data":{"action":{"type":"EXECUTOR"}},"id":"END","position":{"x":800.0,"y":200.0},"type":"END"}]`
	if !StepsEqual(sent, stored) {
		t.Fatal("expected flows to be equal after normalization")
	}
	if StepsEqual(sent, `[{"id":"END","type":"VIEW"}]`) {
		t.Fatal("different flows reported equal")
	}
}
