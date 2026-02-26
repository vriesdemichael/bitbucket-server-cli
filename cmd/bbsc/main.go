package main

import (
	"fmt"
	"os"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/cli"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
)

func main() {
	rootCmd := cli.NewRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(apperrors.ExitCode(err))
	}
}
