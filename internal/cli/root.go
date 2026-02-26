package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/services/repository"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/transport/httpclient"
)

func NewRootCommand() *cobra.Command {
	options := &rootOptions{}

	rootCmd := &cobra.Command{
		Use:           "bbsc",
		Short:         "Bitbucket Server CLI (live-behavior first)",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.PersistentFlags().BoolVar(&options.JSON, "json", false, "Output as JSON")

	rootCmd.AddCommand(newAuthCommand(options))
	rootCmd.AddCommand(newRepoCommand(options))
	rootCmd.AddCommand(newPRCommand(options))
	rootCmd.AddCommand(newIssueCommand(options))
	rootCmd.AddCommand(newAdminCommand(options))

	return rootCmd
}

type rootOptions struct {
	JSON bool
}

func newAuthCommand(options *rootOptions) *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}

	var statusHost string
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show configured target",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(statusHost) != "" {
				if err := os.Setenv("BITBUCKET_URL", statusHost); err != nil {
					return apperrors.New(apperrors.KindInternal, "failed to set host override", err)
				}
			}

			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			if options.JSON {
				payload := map[string]string{
					"bitbucket_url":            cfg.BitbucketURL,
					"bitbucket_version_target": cfg.BitbucketVersionTarget,
					"auth_mode":                cfg.AuthMode(),
					"auth_source":              cfg.AuthSource,
				}
				return writeJSON(cmd.OutOrStdout(), payload)
			}

			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Target Bitbucket: %s (expected version %s, auth=%s, source=%s)\n",
				cfg.BitbucketURL,
				cfg.BitbucketVersionTarget,
				cfg.AuthMode(),
				cfg.AuthSource,
			)
			return nil
		},
	}
	statusCmd.Flags().StringVar(&statusHost, "host", "", "Override host for this status check")
	authCmd.AddCommand(statusCmd)

	var loginHost string
	var loginToken string
	var loginUsername string
	var loginPassword string
	var loginSetDefault bool
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Store credentials for a Bitbucket host",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedHost := strings.TrimSpace(loginHost)
			if resolvedHost == "" {
				cfg, err := config.LoadFromEnv()
				if err != nil {
					return err
				}
				resolvedHost = cfg.BitbucketURL
			}

			result, err := config.SaveLogin(config.LoginInput{
				Host:       resolvedHost,
				Username:   loginUsername,
				Password:   loginPassword,
				Token:      loginToken,
				SetDefault: loginSetDefault,
			})
			if err != nil {
				return err
			}

			if options.JSON {
				payload := map[string]any{
					"host":                  result.Host,
					"auth_mode":             result.AuthMode,
					"used_insecure_storage": result.UsedInsecureStorage,
				}
				return writeJSON(cmd.OutOrStdout(), payload)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Stored credentials for %s (mode=%s)\n", result.Host, result.AuthMode)
			if result.UsedInsecureStorage {
				fmt.Fprintln(cmd.OutOrStdout(), "Warning: keyring unavailable, credentials stored in config fallback")
			}
			return nil
		},
	}
	loginCmd.Flags().StringVar(&loginHost, "host", "", "Bitbucket host URL")
	loginCmd.Flags().StringVar(&loginToken, "token", "", "Access token")
	loginCmd.Flags().StringVar(&loginUsername, "username", "", "Username for basic auth")
	loginCmd.Flags().StringVar(&loginPassword, "password", "", "Password for basic auth")
	loginCmd.Flags().BoolVar(&loginSetDefault, "set-default", true, "Set host as default target")
	authCmd.AddCommand(loginCmd)

	var logoutHost string
	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials for a Bitbucket host",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.Logout(logoutHost); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"status": "ok"})
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Stored credentials removed")
			return nil
		},
	}
	logoutCmd.Flags().StringVar(&logoutHost, "host", "", "Bitbucket host URL (defaults to stored default host)")
	authCmd.AddCommand(logoutCmd)

	return authCmd
}

func newRepoCommand(options *rootOptions) *cobra.Command {
	repoCmd := &cobra.Command{
		Use:   "repo",
		Short: "Repository commands",
	}

	var limit int
	repoCmd.PersistentFlags().IntVar(&limit, "limit", 25, "Page size for Bitbucket list operations")

	repoCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List repositories",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			client := httpclient.NewFromConfig(cfg)
			service := repository.NewService(client)

			repos, err := service.List(cmd.Context(), limit)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), repos)
			}

			if len(repos) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No repositories found")
				return nil
			}

			for _, repo := range repos {
				fmt.Fprintf(cmd.OutOrStdout(), "%s/%s\t%s\n", repo.ProjectKey, repo.Slug, repo.Name)
			}

			return nil
		},
	})

	return repoCmd
}

func newPRCommand(options *rootOptions) *cobra.Command {
	prCmd := &cobra.Command{
		Use:   "pr",
		Short: "Pull request commands",
	}

	prCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List pull requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			return apperrors.New(apperrors.KindNotImplemented, "pr list is not implemented yet", nil)
		},
	})

	return prCmd
}

func newIssueCommand(options *rootOptions) *cobra.Command {
	issueCmd := &cobra.Command{
		Use:   "issue",
		Short: "Issue commands",
	}

	issueCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			return apperrors.New(apperrors.KindNotImplemented, "issue list is not implemented yet", nil)
		},
	})

	return issueCmd
}

func newAdminCommand(options *rootOptions) *cobra.Command {
	adminCmd := &cobra.Command{
		Use:   "admin",
		Short: "Local environment/admin commands",
	}

	adminCmd.AddCommand(&cobra.Command{
		Use:   "health",
		Short: "Check local stack health",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			client := httpclient.NewFromConfig(cfg)
			health, err := client.Health(cmd.Context())
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), health)
			}

			if health.Authenticated {
				fmt.Fprintf(cmd.OutOrStdout(), "Bitbucket health: OK (status=%d, auth=ok)\n", health.StatusCode)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Bitbucket health: OK (status=%d, auth=limited)\n", health.StatusCode)
			return nil
		},
	})

	return adminCmd
}

func writeJSON(writer io.Writer, payload any) error {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "failed to encode JSON output", err)
	}

	fmt.Fprintln(writer, string(encoded))
	return nil
}
