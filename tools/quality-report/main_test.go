package main

import "testing"

func TestCalculatePatchCoverageRequiresAllOverlappingSegmentsCovered(t *testing.T) {
	changed := map[string]map[int]struct{}{
		"internal/example.go": {
			10: {},
		},
	}

	profile := coverageProfile{
		byRelativePath: map[string][]coverageSegment{
			"internal/example.go": {
				{startLine: 10, endLine: 10, covered: true},
				{startLine: 10, endLine: 10, covered: false},
			},
		},
	}

	result := calculatePatchCoverage(changed, profile, []string{"internal/"}, nil)
	if result.coverableLines != 1 {
		t.Fatalf("expected 1 coverable line, got %d", result.coverableLines)
	}
	if result.coveredLines != 0 {
		t.Fatalf("expected 0 covered lines for partial overlap coverage, got %d", result.coveredLines)
	}
}

func TestCalculatePatchCoverageCountsFullyCoveredOverlaps(t *testing.T) {
	changed := map[string]map[int]struct{}{
		"internal/example.go": {
			10: {},
		},
	}

	profile := coverageProfile{
		byRelativePath: map[string][]coverageSegment{
			"internal/example.go": {
				{startLine: 10, endLine: 10, covered: true},
				{startLine: 10, endLine: 10, covered: true},
			},
		},
	}

	result := calculatePatchCoverage(changed, profile, []string{"internal/"}, nil)
	if result.coverableLines != 1 {
		t.Fatalf("expected 1 coverable line, got %d", result.coverableLines)
	}
	if result.coveredLines != 1 {
		t.Fatalf("expected 1 covered line, got %d", result.coveredLines)
	}
}
