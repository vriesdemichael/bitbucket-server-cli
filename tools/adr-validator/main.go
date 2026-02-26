package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var filenamePattern = regexp.MustCompile(`^(\d+)-[a-z0-9-]+\.ya?ml$`)

type rejectedAlternative struct {
	Alternative string `yaml:"alternative"`
	Reason      string `yaml:"reason"`
}

type decisionRecord struct {
	Number               int                   `yaml:"number"`
	Title                string                `yaml:"title"`
	Category             string                `yaml:"category"`
	Status               string                `yaml:"status,omitempty"`
	SupersededBy         *int                  `yaml:"superseded_by,omitempty"`
	Supersedes           any                   `yaml:"supersedes,omitempty"`
	Decision             string                `yaml:"decision"`
	AgentInstructions    string                `yaml:"agent_instructions"`
	Rationale            string                `yaml:"rationale"`
	RejectedAlternatives []rejectedAlternative `yaml:"rejected_alternatives,omitempty"`
	Provenance           string                `yaml:"provenance"`
}

func main() {
	validate := flag.Bool("validate", false, "Validate ADR files (default action)")
	decisionsDir := flag.String("dir", "docs/decisions", "Directory containing ADR YAML files")
	flag.Parse()

	if !*validate {
		*validate = true
	}

	if !*validate {
		fmt.Fprintln(os.Stderr, "no action selected")
		os.Exit(1)
	}

	files, err := resolveFiles(*decisionsDir, flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving files: %v\n", err)
		os.Exit(2)
	}

	if len(files) == 0 {
		fmt.Println("No decision record files found")
		return
	}

	var records []decisionRecord
	for _, path := range files {
		record, err := loadAndValidate(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "VALIDATION ERROR in %s:\n%v\n", path, err)
			os.Exit(2)
		}
		records = append(records, record)
		fmt.Printf("✓ %s\n", path)
	}

	if err := validateNumberSet(records); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func resolveFiles(decisionsDir string, args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}

	pattern := filepath.Join(decisionsDir, "*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

func loadAndValidate(path string) (decisionRecord, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return decisionRecord{}, err
	}

	decoder := yaml.NewDecoder(strings.NewReader(string(raw)))
	decoder.KnownFields(true)

	var record decisionRecord
	if err := decoder.Decode(&record); err != nil {
		if errors.Is(err, io.EOF) {
			return decisionRecord{}, fmt.Errorf("file is empty")
		}
		return decisionRecord{}, err
	}

	if err := validateRecord(record); err != nil {
		return decisionRecord{}, err
	}

	if err := validateFilename(path, record.Number); err != nil {
		return decisionRecord{}, err
	}

	return record, nil
}

func validateRecord(record decisionRecord) error {
	if record.Number <= 0 {
		return fmt.Errorf("number must be > 0")
	}

	if strings.TrimSpace(record.Title) == "" {
		return fmt.Errorf("title is required")
	}

	if !isOneOf(record.Category, []string{"architecture", "development"}) {
		return fmt.Errorf("category must be one of: architecture, development")
	}

	if record.Status != "" && !isOneOf(record.Status, []string{"proposed", "accepted", "superseded", "deprecated"}) {
		return fmt.Errorf("status must be one of: proposed, accepted, superseded, deprecated")
	}

	if record.SupersededBy != nil && *record.SupersededBy <= 0 {
		return fmt.Errorf("superseded_by must be > 0 when set")
	}

	if err := validateSupersedes(record.Supersedes); err != nil {
		return err
	}

	if strings.TrimSpace(record.Decision) == "" {
		return fmt.Errorf("decision is required")
	}

	if strings.TrimSpace(record.AgentInstructions) == "" {
		return fmt.Errorf("agent_instructions is required")
	}

	if strings.TrimSpace(record.Rationale) == "" {
		return fmt.Errorf("rationale is required")
	}

	if !isOneOf(record.Provenance, []string{"human", "guided-ai", "autonomous-ai"}) {
		return fmt.Errorf("provenance must be one of: human, guided-ai, autonomous-ai")
	}

	for i, alternative := range record.RejectedAlternatives {
		if strings.TrimSpace(alternative.Alternative) == "" {
			return fmt.Errorf("rejected_alternatives[%d].alternative is required", i)
		}
		if strings.TrimSpace(alternative.Reason) == "" {
			return fmt.Errorf("rejected_alternatives[%d].reason is required", i)
		}
	}

	return nil
}

func validateSupersedes(value any) error {
	if value == nil {
		return nil
	}

	switch typed := value.(type) {
	case int:
		if typed <= 0 {
			return fmt.Errorf("supersedes must be > 0")
		}
		return nil
	case []any:
		for i, item := range typed {
			number, err := toInt(item)
			if err != nil || number <= 0 {
				return fmt.Errorf("supersedes[%d] must be an integer > 0", i)
			}
		}
		return nil
	default:
		return fmt.Errorf("supersedes must be an integer or list of integers")
	}
}

func validateFilename(path string, number int) error {
	name := filepath.Base(path)
	match := filenamePattern.FindStringSubmatch(name)
	if len(match) == 0 {
		return fmt.Errorf("filename must match NNN-slug.yaml")
	}

	prefix, err := strconv.Atoi(match[1])
	if err != nil {
		return fmt.Errorf("filename number prefix is invalid")
	}

	if prefix != number {
		return fmt.Errorf("filename prefix %03d does not match number %03d", prefix, number)
	}

	return nil
}

func validateNumberSet(records []decisionRecord) error {
	counts := map[int]int{}
	maxNumber := 0

	for _, record := range records {
		counts[record.Number]++
		if record.Number > maxNumber {
			maxNumber = record.Number
		}
	}

	var duplicates []int
	for number, count := range counts {
		if count > 1 {
			duplicates = append(duplicates, number)
		}
	}
	sort.Ints(duplicates)

	if len(duplicates) > 0 {
		return fmt.Errorf("ERROR: Duplicate ADR numbers found: %v", duplicates)
	}

	var missing []int
	for i := 1; i <= maxNumber; i++ {
		if _, ok := counts[i]; !ok {
			missing = append(missing, i)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: Missing ADR numbers: %v\n", missing)
	}

	return nil
}

func toInt(value any) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case float64:
		return int(typed), nil
	default:
		return 0, fmt.Errorf("not an integer")
	}
}

func isOneOf(value string, options []string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}
