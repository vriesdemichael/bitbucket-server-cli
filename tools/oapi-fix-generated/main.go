package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
)

var typeDeclPattern = regexp.MustCompile(`^type\s+([A-Za-z_][A-Za-z0-9_]*)\b`)

func main() {
	filePath := flag.String("file", "", "generated Go file path")
	flag.Parse()

	if *filePath == "" {
		exitWithErr(errors.New("-file is required"))
	}

	if err := dedupeTypeDecls(*filePath); err != nil {
		exitWithErr(err)
	}
}

func dedupeTypeDecls(filePath string) error {
	payload, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(payload))
	lines := make([]string, 0, 4096)
	seenTypes := map[string]struct{}{}
	pendingComment := ""
	removed := 0

	for scanner.Scan() {
		line := scanner.Text()

		if pendingComment != "" {
			if match := typeDeclPattern.FindStringSubmatch(line); len(match) == 2 {
				typeName := match[1]
				if _, exists := seenTypes[typeName]; exists {
					removed++
					pendingComment = ""
					continue
				}
				seenTypes[typeName] = struct{}{}
				lines = append(lines, pendingComment)
				lines = append(lines, line)
				pendingComment = ""
				continue
			}

			lines = append(lines, pendingComment)
			pendingComment = ""
		}

		if isComment(line) {
			pendingComment = line
			continue
		}

		if match := typeDeclPattern.FindStringSubmatch(line); len(match) == 2 {
			typeName := match[1]
			if _, exists := seenTypes[typeName]; exists {
				removed++
				continue
			}
			seenTypes[typeName] = struct{}{}
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan file: %w", err)
	}

	if pendingComment != "" {
		lines = append(lines, pendingComment)
	}

	output := bytes.Join(stringSliceToByteSlices(lines), []byte("\n"))
	output = append(output, '\n')
	if err := os.WriteFile(filePath, output, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Printf("deduplicated generated type declarations; removed=%d\n", removed)
	return nil
}

func isComment(line string) bool {
	return len(line) >= 2 && line[0:2] == "//"
}

func stringSliceToByteSlices(lines []string) [][]byte {
	result := make([][]byte, 0, len(lines))
	for _, line := range lines {
		result = append(result, []byte(line))
	}
	return result
}

func exitWithErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
