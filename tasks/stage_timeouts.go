package tasks

import (
	"fmt"
	"strings"
	"time"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
)

const (
	defaultRetrievalStageTimeout     = 30 * time.Minute
	defaultRetrievalStageTimeoutName = "defaultretrieval"
	defaultStorageStageTimeout       = 3 * time.Hour
	defaultStorageStageTimeoutName   = "defaultstorage"
)

// ParseStageTimeouts parses "StageName=timeout" strings into a map of stage
// name to timeout duration.
func ParseStageTimeouts(timeoutSpecs []string) (map[string]time.Duration, error) {
	// Parse all stage timeout durations
	timeouts := map[string]time.Duration{}
	for _, spec := range timeoutSpecs {
		parts := strings.SplitN(spec, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid stage timeout specification: %s", spec)
		}
		stage := strings.TrimSpace(parts[0])
		timeout := strings.TrimSpace(parts[1])
		d, err := time.ParseDuration(timeout)
		if err != nil || d < time.Second {
			return nil, fmt.Errorf("invalid value for stage %q timeout: %s", stage, timeout)
		}
		stage = strings.ToLower(stage)
		if _, found := timeouts[stage]; found {
			return nil, fmt.Errorf("multiple timeouts specified for stage %q", stage)
		}
		timeouts[stage] = d
	}

	timeoutCount := len(timeouts)

	// Get default stage timeouts
	retrievalTimeout, ok := timeouts[defaultRetrievalStageTimeoutName]
	if !ok {
		retrievalTimeout = defaultRetrievalStageTimeout
		timeoutCount++
	}
	storageTimeout, ok := timeouts[defaultStorageStageTimeoutName]
	if !ok {
		storageTimeout = defaultStorageStageTimeout
		timeoutCount++
	}

	stageTimeouts := make(map[string]time.Duration, timeoutCount)
	stageTimeouts[defaultRetrievalStageTimeoutName] = retrievalTimeout
	stageTimeouts[defaultStorageStageTimeoutName] = storageTimeout

	// Get common stage timeouts
	for stageName, _ := range CommonStages {
		stageName = strings.ToLower(stageName)
		if d, ok := timeouts[stageName]; ok {
			stageTimeouts[stageName] = d
		}
	}

	// Get retrieval stage timeouts
	for stageName, _ := range RetrievalStages {
		stageName = strings.ToLower(stageName)
		if d, ok := timeouts[stageName]; ok {
			stageTimeouts[stageName] = d
		}
	}

	// Get storage stage timeouts
	for _, stageName := range storagemarket.DealStates {
		stageName = strings.ToLower(stageName)
		if d, ok := timeouts[stageName]; ok {
			stageTimeouts[stageName] = d
		}
	}

	// Check for unused stage timeouts
	if len(stageTimeouts) < timeoutCount {
		for stageName, _ := range timeouts {
			if _, ok = stageTimeouts[stageName]; !ok {
				return nil, fmt.Errorf("unusable stage timeout: %q", stageName)
			}
		}
	}

	return stageTimeouts, nil
}
