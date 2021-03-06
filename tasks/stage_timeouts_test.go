package tasks

import (
	"testing"
	"time"
)

func TestParseStageTimeouts(t *testing.T) {
	stageTo, err := ParseStageTimeouts([]string{"ProposeDeal=31m", "DealAccepted=3h", "defaultRetrieval=1h"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stageTo) != 4 {
		t.Fatal("expected 4 retrieval timeouts")
	}
	to, ok := stageTo[defaultStorageStageTimeoutName]
	if !ok {
		t.Fatalf("missing %s timeout", defaultStorageStageTimeoutName)
	}
	if to != defaultStorageStageTimeout {
		t.Errorf("wrong value for %s timeout", defaultStorageStageTimeoutName)
	}

	to, ok = stageTo["proposedeal"]
	if !ok {
		t.Fatal("missing proposedeal timeout")
	}
	if to != 31*time.Minute {
		t.Error("wrong value for proposedeal timeout")
	}

	to, ok = stageTo["dealaccepted"]
	if !ok {
		t.Fatal("missing dealaccepted timeout")
	}
	if to != 3*time.Hour {
		t.Error("wrong value for dealaccepted timeout")
	}

	stageTo, err = ParseStageTimeouts([]string{"DealAccepted=3x"})
	if err == nil {
		t.Fatal("expected error from bad duration value")
	}

	stageTo, err = ParseStageTimeouts([]string{"DealAccepted:3h"})
	if err == nil {
		t.Fatal("expected error from bad specification")
	}

	stageTo, err = ParseStageTimeouts([]string{"unknown=3h"})
	if err == nil {
		t.Fatal("expected error with unusable stage timeout")
	}
}
