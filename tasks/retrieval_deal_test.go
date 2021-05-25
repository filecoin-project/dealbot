package tasks

import (
	"testing"
	"time"
)

func TestParseStageTimeouts(t *testing.T) {
	retTo, stoTo, err := ParseStageTimeouts([]string{"ProposeRetrieval=31m", "DealAccepted=3h"})
	if err != nil {
		t.Fatal(err)
	}
	if stoTo != nil {
		t.Error("not expecteing storage timeouts")
	}
	if len(retTo) != 2 {
		t.Fatal("expected 2 retrieval timeouts")
	}
	to, ok := retTo["proposeretrieval"]
	if !ok {
		t.Fatal("missing proposeretrieval timeout")
	}
	if to != 31*time.Minute {
		t.Error("wrong value for proposeretrieval timeout")
	}

	to, ok = retTo["dealaccepted"]
	if !ok {
		t.Fatal("missing dealaccepted timeout")
	}
	if to != 3*time.Hour {
		t.Error("wrong value for dealaccepted timeout")
	}

	retTo, _, err = ParseStageTimeouts([]string{"DealAccepted=3x"})
	if err == nil {
		t.Fatal("expected error from bad duration value")
	}

	retTo, _, err = ParseStageTimeouts([]string{"DealAccepted:3h"})
	if err == nil {
		t.Fatal("expected error from bad specification")
	}

	retTo, _, err = ParseStageTimeouts([]string{"unknown=3h"})
	if err != nil {
		t.Fatal(err)
	}
	if retTo != nil {
		t.Fatal("should not have retrieval timeouts")
	}
}
