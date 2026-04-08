package cli

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/cli/style"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	githubrelease "github.com/vriesdemichael/bitbucket-server-cli/internal/transport/githubrelease"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/transport/network"
	updateworkflow "github.com/vriesdemichael/bitbucket-server-cli/internal/workflows/update"
)

var updateRunnerFactory = func(version string) *updateworkflow.Runner {
	transport, err := network.NewSafeTransport(network.TLSOptions{})
	if err != nil {
		transport = &network.SafeTransport{}
	}

	client := githubrelease.NewClient(
		"https://api.github.com",
		&http.Client{Timeout: 30 * time.Second, Transport: transport},
		fmt.Sprintf("bb/%s", strings.TrimSpace(version)),
	)

	return updateworkflow.NewRunner(updateworkflow.Dependencies{
		Releases:        client,
		RepositoryOwner: "vriesdemichael",
		RepositoryName:  "bitbucket-server-cli",
		CurrentVersion:  func() string { return strings.TrimSpace(version) },
		ExecutablePath:  os.Executable,
		Platform:        func() (string, string) { return runtime.GOOS, runtime.GOARCH },
	})
}

func newUpdateCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Check for and install the latest bb release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if options == nil {
				return apperrors.New(apperrors.KindInternal, "update command options are not configured", nil)
			}

			runner := updateRunnerFactory(cmd.Root().Version)
			result, err := runner.Run(cmd.Context(), updateworkflow.Options{DryRun: options.DryRun})
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), result)
			}

			writeUpdateHuman(cmd, result)
			return nil
		},
	}
}

func writeUpdateHuman(cmd *cobra.Command, result updateworkflow.Result) {
	if cmd == nil {
		return
	}

	writer := cmd.OutOrStdout()
	if result.DryRun {
		fmt.Fprintf(writer, "%s\n", style.DryRun.Render("Dry-run (static, capability=full)"))
	}

	switch {
	case result.UpToDate:
		fmt.Fprintf(writer, "%s %s\n", style.Success.Render("bb is up to date"), style.Resource.Render(result.CurrentVersion))
	case result.Applied:
		fmt.Fprintf(writer, "%s %s %s %s\n", style.Updated.Render("Updated bb"), style.Secondary.Render(result.CurrentVersion), style.Secondary.Render("->"), style.Resource.Render(result.LatestVersion))
	case result.UpdateAvailable && result.DryRun:
		fmt.Fprintf(writer, "%s %s %s %s\n", style.Warning.Render("Update available"), style.Secondary.Render(result.CurrentVersion), style.Secondary.Render("->"), style.Resource.Render(result.LatestVersion))
	case result.UpdateAvailable:
		fmt.Fprintf(writer, "%s %s %s %s\n", style.Warning.Render("Update available"), style.Secondary.Render(result.CurrentVersion), style.Secondary.Render("->"), style.Resource.Render(result.LatestVersion))
	default:
		fmt.Fprintf(writer, "%s %s\n", style.Secondary.Render("Current version"), style.Resource.Render(result.CurrentVersion))
	}

	if result.AssetName != "" {
		fmt.Fprintf(writer, "%s %s\n", style.Secondary.Render("artifact"), result.AssetName)
	}
	if result.InstallPath != "" {
		fmt.Fprintf(writer, "%s %s\n", style.Secondary.Render("install_path"), result.InstallPath)
	}
	if result.ChecksumAssetName != "" {
		status := "available"
		if result.ChecksumVerified {
			status = "verified"
		}
		fmt.Fprintf(writer, "%s %s (%s)\n", style.Secondary.Render("checksum"), result.ChecksumAssetName, status)
	}
	if result.ReleaseURL != "" {
		fmt.Fprintf(writer, "%s %s\n", style.Secondary.Render("release"), result.ReleaseURL)
	}
	if result.DryRun && result.PlannedAction != "" {
		fmt.Fprintf(writer, "%s %s\n", style.Secondary.Render("planned_action"), result.PlannedAction)
	}
}
