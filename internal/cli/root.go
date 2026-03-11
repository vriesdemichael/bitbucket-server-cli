package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	authcmd "github.com/vriesdemichael/bitbucket-server-cli/internal/cli/cmd/auth"
	bulkcmd "github.com/vriesdemichael/bitbucket-server-cli/internal/cli/cmd/bulk"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/diagnostics"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/openapi"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
	qualityservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/quality"
)

func NewRootCommand() *cobra.Command {
	options := &rootOptions{}

	rootCmd := &cobra.Command{
		Use:           "bbsc",
		Short:         "Bitbucket Server CLI (live-behavior first)",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			diagnostics.SetOutputWriter(cmd.ErrOrStderr())
			if err := applyRuntimeFlagOverrides(cmd); err != nil {
				return err
			}

			return applyInferredRepositoryContext(cmd, options.JSON)
		},
	}

	rootCmd.PersistentFlags().BoolVar(&options.JSON, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().String("ca-file", "", "Path to PEM CA bundle for TLS trust")
	rootCmd.PersistentFlags().Bool("insecure-skip-verify", false, "Disable TLS certificate verification (unsafe; local/dev only)")
	rootCmd.PersistentFlags().String("request-timeout", "", "HTTP request timeout (Go duration, e.g. 20s)")
	rootCmd.PersistentFlags().Int("retry-count", -1, "HTTP retry attempts for transient errors")
	rootCmd.PersistentFlags().String("retry-backoff", "", "Base retry backoff duration (e.g. 250ms)")
	rootCmd.PersistentFlags().String("log-level", "", "Diagnostics verbosity: error, warn, info, debug")
	rootCmd.PersistentFlags().String("log-format", "", "Diagnostics format: text or jsonl")

	rootCmd.AddCommand(authcmd.New(authcmd.Dependencies{
		JSONEnabled: func() bool { return options.JSON },
		LoadConfig:  loadConfig,
		WriteJSON:   writeJSON,
	}))
	rootCmd.AddCommand(bulkcmd.New(bulkcmd.Dependencies{
		JSONEnabled: func() bool { return options.JSON },
		LoadConfig:  loadConfig,
		WriteJSON:   writeJSON,
	}))
	rootCmd.AddCommand(newRepoCommand(options))
	rootCmd.AddCommand(newTagCommand(options))
	rootCmd.AddCommand(newBranchCommand(options))
	rootCmd.AddCommand(newDiffCommand(options))
	rootCmd.AddCommand(newBuildCommand(options))
	rootCmd.AddCommand(newInsightsCommand(options))
	rootCmd.AddCommand(newPRCommand(options))
	rootCmd.AddCommand(newAdminCommand(options))
	rootCmd.AddCommand(newCommitCommand(options))
	rootCmd.AddCommand(newRefCommand(options))
	rootCmd.AddCommand(newProjectCommand(options))
	rootCmd.AddCommand(newReviewerCommand(options))
	rootCmd.AddCommand(newHookCommand(options))
	rootCmd.AddCommand(newSearchCommand(options))

	return rootCmd
}

type rootOptions struct {
	JSON bool
}

func loadConfig() (config.AppConfig, error) {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return config.AppConfig{}, err
	}

	if cfg.InsecureSkipVerify {
		insecureTLSWarningOnce.Do(func() {
			fmt.Fprintln(os.Stderr, "Warning: TLS certificate verification is disabled (--insecure-skip-verify / BBSC_INSECURE_SKIP_VERIFY); use only for local or development environments")
		})
	}

	return cfg, nil
}

var insecureTLSWarningOnce sync.Once

func applyRuntimeFlagOverrides(cmd *cobra.Command) error {
	if cmd == nil {
		return nil
	}

	lookupFlag := func(flagName string) *pflag.Flag {
		if flag := cmd.Flags().Lookup(flagName); flag != nil {
			return flag
		}
		return cmd.PersistentFlags().Lookup(flagName)
	}

	setIfChanged := func(flagName, envKey string) error {
		flag := lookupFlag(flagName)
		if flag == nil || !flag.Changed {
			return nil
		}

		value := strings.TrimSpace(flag.Value.String())

		if value == "" {
			if err := os.Unsetenv(envKey); err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to clear runtime override", err)
			}
			return nil
		}

		if err := os.Setenv(envKey, value); err != nil {
			return apperrors.New(apperrors.KindInternal, "failed to set runtime override", err)
		}

		return nil
	}

	if err := setIfChanged("ca-file", "BBSC_CA_FILE"); err != nil {
		return err
	}

	if err := setIfChanged("insecure-skip-verify", "BBSC_INSECURE_SKIP_VERIFY"); err != nil {
		return err
	}

	if err := setIfChanged("request-timeout", "BBSC_REQUEST_TIMEOUT"); err != nil {
		return err
	}

	if err := setIfChanged("retry-count", "BBSC_RETRY_COUNT"); err != nil {
		return err
	}

	if err := setIfChanged("retry-backoff", "BBSC_RETRY_BACKOFF"); err != nil {
		return err
	}

	if err := setIfChanged("log-level", "BBSC_LOG_LEVEL"); err != nil {
		return err
	}

	if err := setIfChanged("log-format", "BBSC_LOG_FORMAT"); err != nil {
		return err
	}

	return nil
}

func loadConfigAndClient() (config.AppConfig, *openapigenerated.ClientWithResponses, error) {
	cfg, err := loadConfig()
	if err != nil {
		return config.AppConfig{}, nil, err
	}

	client, err := newAPIClientFromConfig(cfg)
	if err != nil {
		return config.AppConfig{}, nil, apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
	}

	return cfg, client, nil
}

func loadQualityRepoAndService(selector string) (qualityservice.RepositoryRef, *qualityservice.Service, error) {
	cfg, client, err := loadConfigAndClient()
	if err != nil {
		return qualityservice.RepositoryRef{}, nil, err
	}

	repo, err := resolveQualityRepositoryReference(selector, cfg)
	if err != nil {
		return qualityservice.RepositoryRef{}, nil, err
	}

	return repo, qualityservice.NewService(client), nil
}

func newAPIClientFromConfig(cfg config.AppConfig) (*openapigenerated.ClientWithResponses, error) {
	return openapi.NewClientWithResponsesFromConfig(cfg)
}

func writeJSON(writer io.Writer, payload any) error {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "failed to encode JSON output", err)
	}

	fmt.Fprintln(writer, string(encoded))
	return nil
}
