package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type coverageSegment struct {
	startLine int
	endLine   int
	numStmt   int
	covered   bool
}

type coverageProfile struct {
	totalStatements   int
	coveredStatements int
	byRelativePath    map[string][]coverageSegment
}

type patchCoverage struct {
	coverableLines int
	coveredLines   int
	percent        float64
}

func main() {
	coverProfilePath := flag.String("coverprofile", ".tmp/coverage.out", "Path to go coverage profile")
	baseRef := flag.String("base-ref", "main", "Base git ref used for patch coverage diff")
	minTotal := flag.Float64("min-total", 85.0, "Minimum required global coverage percentage")
	minPatch := flag.Float64("min-patch", 85.0, "Minimum required patch coverage percentage")
	globalScope := flag.String("global-scope", "changed", "Global coverage scope: changed or all")
	flag.Parse()

	modulePath, err := readModulePath("go.mod")
	if err != nil {
		fail("failed to read module path: %v", err)
	}

	profile, err := parseCoverageProfile(*coverProfilePath, modulePath)
	if err != nil {
		fail("failed to parse coverage profile: %v", err)
	}

	changedLines, err := collectChangedLines(*baseRef)
	if err != nil {
		fail("failed to collect changed lines: %v", err)
	}

	totalCovered := profile.coveredStatements
	totalStatements := profile.totalStatements
	if strings.EqualFold(strings.TrimSpace(*globalScope), "changed") {
		totalCovered, totalStatements = calculateChangedScopeTotals(changedLines, profile)
	}

	totalPercent := percent(totalCovered, totalStatements)

	patch := calculatePatchCoverage(changedLines, profile)

	if strings.EqualFold(strings.TrimSpace(*globalScope), "changed") {
		fmt.Printf("Global coverage (changed scope): %.2f%% (%d/%d statements)\n", totalPercent, totalCovered, totalStatements)
	} else {
		fmt.Printf("Global coverage: %.2f%% (%d/%d statements)\n", totalPercent, totalCovered, totalStatements)
	}
	if patch.coverableLines == 0 {
		fmt.Println("Patch coverage: 100.00% (no coverable changed lines)")
	} else {
		fmt.Printf("Patch coverage: %.2f%% (%d/%d changed lines)\n", patch.percent, patch.coveredLines, patch.coverableLines)
	}

	var failed bool
	if totalPercent < *minTotal {
		fmt.Printf("FAIL: global coverage %.2f%% is below required %.2f%%\n", totalPercent, *minTotal)
		failed = true
	}
	if patch.percent < *minPatch {
		fmt.Printf("FAIL: patch coverage %.2f%% is below required %.2f%%\n", patch.percent, *minPatch)
		failed = true
	}

	if failed {
		os.Exit(1)
	}
}

func parseCoverageProfile(path string, modulePath string) (coverageProfile, error) {
	file, err := os.Open(path)
	if err != nil {
		return coverageProfile{}, err
	}
	defer file.Close()

	profile := coverageProfile{byRelativePath: map[string][]coverageSegment{}}
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "mode:") {
			continue
		}

		pathPart, rangePart, numStmt, count, err := parseCoverageLine(line)
		if err != nil {
			return coverageProfile{}, fmt.Errorf("line %d: %w", lineNumber, err)
		}

		relPath := pathPart
		modulePrefix := modulePath + "/"
		if strings.HasPrefix(relPath, modulePrefix) {
			relPath = strings.TrimPrefix(relPath, modulePrefix)
		}
		relPath = filepath.ToSlash(relPath)

		profile.totalStatements += numStmt
		covered := count > 0
		if covered {
			profile.coveredStatements += numStmt
		}

		segment := coverageSegment{
			startLine: rangePart.startLine,
			endLine:   rangePart.endLine,
			numStmt:   numStmt,
			covered:   covered,
		}
		profile.byRelativePath[relPath] = append(profile.byRelativePath[relPath], segment)
	}

	if err := scanner.Err(); err != nil {
		return coverageProfile{}, err
	}

	if profile.totalStatements == 0 {
		return coverageProfile{}, errors.New("no statements found in coverage profile")
	}

	return profile, nil
}

type segmentRange struct {
	startLine int
	endLine   int
}

func parseCoverageLine(line string) (string, segmentRange, int, int, error) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return "", segmentRange{}, 0, 0, fmt.Errorf("invalid coverage line format: %q", line)
	}

	location := fields[0]
	numStmt, err := strconv.Atoi(fields[1])
	if err != nil {
		return "", segmentRange{}, 0, 0, fmt.Errorf("invalid statement count: %w", err)
	}
	count, err := strconv.Atoi(fields[2])
	if err != nil {
		return "", segmentRange{}, 0, 0, fmt.Errorf("invalid execution count: %w", err)
	}

	colonIndex := strings.LastIndex(location, ":")
	if colonIndex == -1 {
		return "", segmentRange{}, 0, 0, fmt.Errorf("missing location separator in %q", location)
	}

	pathPart := location[:colonIndex]
	rangePart := location[colonIndex+1:]
	parts := strings.Split(rangePart, ",")
	if len(parts) != 2 {
		return "", segmentRange{}, 0, 0, fmt.Errorf("invalid range in %q", rangePart)
	}

	startParts := strings.Split(parts[0], ".")
	endParts := strings.Split(parts[1], ".")
	if len(startParts) != 2 || len(endParts) != 2 {
		return "", segmentRange{}, 0, 0, fmt.Errorf("invalid position in %q", rangePart)
	}

	startLine, err := strconv.Atoi(startParts[0])
	if err != nil {
		return "", segmentRange{}, 0, 0, fmt.Errorf("invalid start line: %w", err)
	}
	endLine, err := strconv.Atoi(endParts[0])
	if err != nil {
		return "", segmentRange{}, 0, 0, fmt.Errorf("invalid end line: %w", err)
	}

	return pathPart, segmentRange{startLine: startLine, endLine: endLine}, numStmt, count, nil
}

func collectChangedLines(baseRef string) (map[string]map[int]struct{}, error) {
	mergeBaseCmd := exec.Command("git", "merge-base", baseRef, "HEAD")
	mergeBaseOutput, err := mergeBaseCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git merge-base failed: %w", err)
	}
	mergeBase := strings.TrimSpace(string(mergeBaseOutput))
	if mergeBase == "" {
		return nil, errors.New("empty merge-base result")
	}

	diffCmd := exec.Command("git", "diff", "--unified=0", "--no-color", mergeBase, "--", ".")
	diffOutput, err := diffCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	return parseUnifiedDiffChangedLines(string(diffOutput)), nil
}

func parseUnifiedDiffChangedLines(diff string) map[string]map[int]struct{} {
	changed := map[string]map[int]struct{}{}
	lines := strings.Split(diff, "\n")
	currentFile := ""
	currentNewLine := 0
	inHunk := false
	hunkPattern := regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

	for _, line := range lines {
		if strings.HasPrefix(line, "+++ ") {
			inHunk = false
			fileToken := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			if fileToken == "/dev/null" {
				currentFile = ""
				continue
			}
			currentFile = strings.TrimPrefix(fileToken, "b/")
			currentFile = filepath.ToSlash(currentFile)
			if !strings.HasSuffix(currentFile, ".go") {
				currentFile = ""
			}
			continue
		}

		if strings.HasPrefix(line, "@@") {
			matches := hunkPattern.FindStringSubmatch(line)
			if len(matches) == 0 {
				inHunk = false
				continue
			}
			startLine, err := strconv.Atoi(matches[1])
			if err != nil {
				inHunk = false
				continue
			}
			currentNewLine = startLine
			inHunk = true
			continue
		}

		if !inHunk || currentFile == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			if !strings.HasSuffix(currentFile, "_test.go") {
				if _, ok := changed[currentFile]; !ok {
					changed[currentFile] = map[int]struct{}{}
				}
				changed[currentFile][currentNewLine] = struct{}{}
			}
			currentNewLine++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			// deleted line: does not advance new-side line number
		case strings.HasPrefix(line, " "):
			currentNewLine++
		default:
			inHunk = false
		}
	}

	return changed
}

func calculatePatchCoverage(changed map[string]map[int]struct{}, profile coverageProfile) patchCoverage {
	result := patchCoverage{}

	for filePath, lineSet := range changed {
		segments := profile.byRelativePath[filePath]
		if len(segments) == 0 {
			continue
		}

		for line := range lineSet {
			coverable := false
			covered := false
			for _, segment := range segments {
				if line < segment.startLine || line > segment.endLine {
					continue
				}
				coverable = true
				if segment.covered {
					covered = true
					break
				}
			}
			if !coverable {
				continue
			}
			result.coverableLines++
			if covered {
				result.coveredLines++
			}
		}
	}

	if result.coverableLines == 0 {
		result.percent = 100.0
		return result
	}

	result.percent = percent(result.coveredLines, result.coverableLines)
	return result
}

func calculateChangedScopeTotals(changed map[string]map[int]struct{}, profile coverageProfile) (int, int) {
	coveredTotal := 0
	statementTotal := 0

	for filePath := range changed {
		segments := profile.byRelativePath[filePath]
		if len(segments) == 0 {
			continue
		}

		for _, segment := range segments {
			statementTotal += segment.numStmt
			if segment.covered {
				coveredTotal += segment.numStmt
			}
		}
	}

	return coveredTotal, statementTotal
}

func readModulePath(goModPath string) (string, error) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}

	for _, rawLine := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "module ") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if value == "" {
				break
			}
			return value, nil
		}
	}

	return "", errors.New("module declaration not found")
}

func percent(covered int, total int) float64 {
	if total <= 0 {
		return 100.0
	}
	return (float64(covered) / float64(total)) * 100.0
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
