package auth

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
)

type Dependencies struct {
	JSONEnabled func() bool
	LoadConfig  func() (config.AppConfig, error)
	WriteJSON   func(io.Writer, any) error
}

func New(deps Dependencies) *cobra.Command {
	if deps.LoadConfig == nil {
		deps.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{}, apperrors.New(apperrors.KindInternal, "auth command dependency LoadConfig is not configured", nil)
		}
	}

	if deps.WriteJSON == nil {
		deps.WriteJSON = func(io.Writer, any) error {
			return apperrors.New(apperrors.KindInternal, "auth command dependency WriteJSON is not configured", nil)
		}
	}

	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}

	isJSON := func() bool {
		if deps.JSONEnabled == nil {
			return false
		}
		return deps.JSONEnabled()
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

			cfg, err := deps.LoadConfig()
			if err != nil {
				return err
			}

			if isJSON() {
				payload := map[string]string{
					"bitbucket_url":            cfg.BitbucketURL,
					"bitbucket_version_target": cfg.BitbucketVersionTarget,
					"auth_mode":                cfg.AuthMode(),
					"auth_source":              cfg.AuthSource,
				}
				return deps.WriteJSON(cmd.OutOrStdout(), payload)
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
				cfg, err := deps.LoadConfig()
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

			if isJSON() {
				payload := map[string]any{
					"host":                  result.Host,
					"auth_mode":             result.AuthMode,
					"used_insecure_storage": result.UsedInsecureStorage,
				}
				return deps.WriteJSON(cmd.OutOrStdout(), payload)
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

			if isJSON() {
				return deps.WriteJSON(cmd.OutOrStdout(), map[string]string{"status": "ok"})
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Stored credentials removed")
			return nil
		},
	}
	logoutCmd.Flags().StringVar(&logoutHost, "host", "", "Bitbucket host URL (defaults to stored default host)")
	authCmd.AddCommand(logoutCmd)

	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Manage server contexts",
	}

	serverListCmd := &cobra.Command{
		Use:   "list",
		Short: "List stored server contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			contexts, err := config.ListServerContexts()
			if err != nil {
				return err
			}

			if isJSON() {
				payload := map[string]any{
					"servers": contexts,
				}
				return deps.WriteJSON(cmd.OutOrStdout(), payload)
			}

			if len(contexts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stored server contexts")
				return nil
			}

			for _, context := range contexts {
				marker := " "
				if context.IsDefault {
					marker = "*"
				}

				if context.Username != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s %s (auth=%s, user=%s)\n", marker, context.Host, context.AuthMode, context.Username)
					continue
				}

				fmt.Fprintf(cmd.OutOrStdout(), "%s %s (auth=%s)\n", marker, context.Host, context.AuthMode)
			}

			return nil
		},
	}
	serverCmd.AddCommand(serverListCmd)

	var serverUseHost string
	serverUseCmd := &cobra.Command{
		Use:   "use [host]",
		Short: "Set the active default server context",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(serverUseHost) == "" && len(args) > 0 {
				serverUseHost = args[0]
			}

			resolvedHost, err := config.SetDefaultHost(serverUseHost)
			if err != nil {
				return err
			}

			if isJSON() {
				return deps.WriteJSON(cmd.OutOrStdout(), map[string]string{
					"status":       "ok",
					"default_host": resolvedHost,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Active server set to %s\n", resolvedHost)
			return nil
		},
	}
	serverUseCmd.Flags().StringVar(&serverUseHost, "host", "", "Bitbucket host URL")
	serverCmd.AddCommand(serverUseCmd)

	authCmd.AddCommand(serverCmd)

	return authCmd
}
