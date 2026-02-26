package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var pathParameterPattern = regexp.MustCompile(`\{([^{}]+)\}`)
var nonAlnumPattern = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func main() {
	inputPath := flag.String("in", "", "input OpenAPI JSON path")
	outputPath := flag.String("out", "", "output OpenAPI JSON path")
	flag.Parse()

	if *inputPath == "" || *outputPath == "" {
		exitWithErr(errors.New("both -in and -out are required"))
	}

	if err := sanitize(*inputPath, *outputPath); err != nil {
		exitWithErr(err)
	}
}

func sanitize(inputPath, outputPath string) error {
	payload, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	var spec map[string]any
	if err := json.Unmarshal(payload, &spec); err != nil {
		return fmt.Errorf("unmarshal spec: %w", err)
	}

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		return errors.New("spec paths missing or invalid")
	}

	fixedOperations := 0
	renamedOperationIDs := 0
	seenOperationIDs := map[string]int{}
	for rawPath, rawPathItem := range paths {
		pathItem, ok := rawPathItem.(map[string]any)
		if !ok {
			continue
		}

		requiredParams := requiredPathParams(rawPath)

		for _, method := range []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"} {
			rawOp, exists := pathItem[method]
			if !exists {
				continue
			}

			op, ok := rawOp.(map[string]any)
			if !ok {
				continue
			}

			operationID, _ := op["operationId"].(string)
			if operationID == "" {
				operationID = operationIDFromPath(method, rawPath)
				op["operationId"] = operationID
			}

			canonicalID := canonicalOperationID(operationID)
			if seenOperationIDs[canonicalID] > 0 {
				renamedOperationIDs++
				operationID = fmt.Sprintf("%s_%d", operationID, seenOperationIDs[canonicalID]+1)
				op["operationId"] = operationID
				canonicalID = canonicalOperationID(operationID)
			}
			seenOperationIDs[canonicalID]++

			declared := declaredPathParamNames(op["parameters"])
			added := false
			for _, name := range requiredParams {
				if _, present := declared[name]; present {
					continue
				}
				op["parameters"] = appendParameter(op["parameters"], name)
				declared[name] = struct{}{}
				added = true
			}
			if added {
				fixedOperations++
			}
		}
	}

	output, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sanitized spec: %w", err)
	}

	if err := os.WriteFile(outputPath, output, 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	fmt.Printf("sanitized OpenAPI spec; fixed operations=%d renamed operationIds=%d\n", fixedOperations, renamedOperationIDs)
	return nil
}

func operationIDFromPath(method, path string) string {
	normalizedPath := nonAlnumPattern.ReplaceAllString(path, "_")
	normalizedPath = strings.Trim(normalizedPath, "_")
	if normalizedPath == "" {
		normalizedPath = "root"
	}
	return fmt.Sprintf("%s_%s", strings.ToLower(method), normalizedPath)
}

func canonicalOperationID(operationID string) string {
	normalized := nonAlnumPattern.ReplaceAllString(operationID, "")
	return strings.ToLower(normalized)
}

func requiredPathParams(path string) []string {
	matches := pathParameterPattern.FindAllStringSubmatch(path, -1)
	if len(matches) == 0 {
		return nil
	}

	params := make([]string, 0, len(matches))
	seen := map[string]struct{}{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := match[1]
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		params = append(params, name)
	}
	return params
}

func declaredPathParamNames(rawParameters any) map[string]struct{} {
	declared := map[string]struct{}{}

	parameters, ok := rawParameters.([]any)
	if !ok {
		return declared
	}

	for _, rawParameter := range parameters {
		parameter, ok := rawParameter.(map[string]any)
		if !ok {
			continue
		}
		if parameter["in"] != "path" {
			continue
		}
		name, ok := parameter["name"].(string)
		if !ok || name == "" {
			continue
		}
		declared[name] = struct{}{}
	}

	return declared
}

func appendParameter(rawParameters any, name string) []any {
	parameter := map[string]any{
		"name":     name,
		"in":       "path",
		"required": true,
		"schema": map[string]any{
			"type": "string",
		},
	}

	if rawParameters == nil {
		return []any{parameter}
	}

	parameters, ok := rawParameters.([]any)
	if !ok {
		return []any{parameter}
	}

	return append(parameters, parameter)
}

func exitWithErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
