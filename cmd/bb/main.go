package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/cli"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/diagnostics"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
)

// Version is set at build time via -ldflags "-X main.Version=<semver>".
var Version = "dev"

func main() {
	cmd := cli.NewRootCommand()
	cmd.Version = Version
	os.Exit(executeRootCommand(cmd, os.Stderr))
}

func executeRootCommand(rootCmd *cobra.Command, stderr io.Writer) int {
	if stderr == nil {
		stderr = io.Discard
	}

	if err := rootCmd.Execute(); err != nil {
		emitCommandFailureDiagnostic(err, stderr)
		fmt.Fprintln(stderr, err.Error())
		return apperrors.ExitCode(err)
	}

	return 0
}

func emitCommandFailureDiagnostic(err error, stderr io.Writer) {
	logger, enabled := loggerFromEnvironment(stderr)
	if !enabled {
		return
	}

	logger.Error("command execution failed", map[string]any{
		"correlation_id": newCorrelationID(),
		"error_kind":     apperrors.KindOf(err),
		"exit_code":      apperrors.ExitCode(err),
		"error":          err.Error(),
	})
}

func loggerFromEnvironment(stderr io.Writer) (*diagnostics.Logger, bool) {
	rawLevel := strings.TrimSpace(os.Getenv("BB_LOG_LEVEL"))
	rawFormat := strings.TrimSpace(os.Getenv("BB_LOG_FORMAT"))
	enabled := rawLevel != "" || rawFormat != ""
	if !enabled {
		return diagnostics.NewLogger(diagnostics.Config{}, io.Discard), false
	}

	level := rawLevel
	if level == "" {
		level = string(diagnostics.LevelError)
	}
	parsedLevel, levelErr := diagnostics.ParseLevel(level)
	if levelErr != nil {
		return diagnostics.NewLogger(diagnostics.Config{}, io.Discard), false
	}

	format := rawFormat
	if format == "" {
		format = string(diagnostics.FormatText)
	}
	parsedFormat, formatErr := diagnostics.ParseFormat(format)
	if formatErr != nil {
		return diagnostics.NewLogger(diagnostics.Config{}, io.Discard), false
	}

	return diagnostics.NewLogger(diagnostics.Config{Level: parsedLevel, Format: parsedFormat}, stderr), true
}

func newCorrelationID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return ""
	}

	return hex.EncodeToString(buffer)
}
