package auth

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/openapi"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

type usersClient interface {
	GetUsers2WithResponse(ctx context.Context, params *openapigenerated.GetUsers2Params, reqEditors ...openapigenerated.RequestEditorFn) (*openapigenerated.GetUsers2Response, error)
}

type Dependencies struct {
	JSONEnabled    func() bool
	LoadConfig     func() (config.AppConfig, error)
	WriteJSON      func(io.Writer, any) error
	NewUsersClient func(config.AppConfig) (usersClient, error)
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

	if deps.NewUsersClient == nil {
		deps.NewUsersClient = func(cfg config.AppConfig) (usersClient, error) {
			return openapi.NewClientWithResponsesFromConfig(cfg)
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

	var loginToken string
	var loginUsername string
	var loginPassword string
	var loginSetDefault bool
	loginCmd := &cobra.Command{
		Use:   "login <host>",
		Short: "Store credentials for a Bitbucket host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedHost := strings.TrimSpace(args[0])

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
	loginCmd.Flags().StringVar(&loginToken, "token", "", "Access token")
	loginCmd.Flags().StringVar(&loginUsername, "username", "", "Username for basic auth")
	loginCmd.Flags().StringVar(&loginPassword, "password", "", "Password for basic auth")
	loginCmd.Flags().BoolVar(&loginSetDefault, "set-default", true, "Set host as default target")
	authCmd.AddCommand(loginCmd)

	var identityHost string
	identityCmd := &cobra.Command{
		Use:     "identity",
		Aliases: []string{"whoami"},
		Short:   "Show authenticated user identity",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigWithOptionalHostOverride(deps.LoadConfig, identityHost)
			if err != nil {
				return err
			}

			identity, err := resolveIdentity(cmd.Context(), cfg, deps.NewUsersClient)
			if err != nil {
				return err
			}

			if isJSON() {
				return deps.WriteJSON(cmd.OutOrStdout(), map[string]any{
					"bitbucket_url": cfg.BitbucketURL,
					"user":          identity,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Authenticated user: %s\n", identityHumanSummary(identity))
			return nil
		},
	}
	identityCmd.Flags().StringVar(&identityHost, "host", "", "Override host for this identity check")
	authCmd.AddCommand(identityCmd)

	var tokenHost string
	tokenCmd := &cobra.Command{
		Use:   "token-url",
		Short: "Show personal access token creation URL",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config once. If --host is provided, override the URL so the identity
			// lookup targets the same server as the PAT URL being generated.
			cfg, err := deps.LoadConfig()
			if err != nil {
				return err
			}

			resolvedHost := strings.TrimSpace(tokenHost)
			if resolvedHost == "" {
				resolvedHost = cfg.BitbucketURL
			} else {
				// Apply --host override so identity resolution targets the right server.
				cfg.BitbucketURL = resolvedHost
			}

			// Attempt to resolve the current user slug for a per-user PAT URL.
			// If credentials are not configured, fall back to the generic URL.
			var userSlug string
			if cfg.AuthMode() != "none" {
				if identity, err := resolveIdentity(cmd.Context(), cfg, deps.NewUsersClient); err == nil {
					userSlug = identity.Slug
				}
			}

			patURL, err := personalAccessTokenURL(resolvedHost, userSlug)
			if err != nil {
				return err
			}

			if isJSON() {
				return deps.WriteJSON(cmd.OutOrStdout(), map[string]string{
					"bitbucket_url": resolvedHost,
					"token_url":     patURL,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Create a personal access token at:\n%s\n", patURL)
			return nil
		},
	}
	tokenCmd.Flags().StringVar(&tokenHost, "host", "", "Bitbucket host URL")
	authCmd.AddCommand(tokenCmd)

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

type authIdentity struct {
	Name        string `json:"name,omitempty"`
	Slug        string `json:"slug,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	ID          int64  `json:"id,omitempty"`
	Type        string `json:"type,omitempty"`
	Active      bool   `json:"active"`
}

func loadConfigWithOptionalHostOverride(loadConfig func() (config.AppConfig, error), hostOverride string) (config.AppConfig, error) {
	if strings.TrimSpace(hostOverride) != "" {
		if err := os.Setenv("BITBUCKET_URL", hostOverride); err != nil {
			return config.AppConfig{}, apperrors.New(apperrors.KindInternal, "failed to set host override", err)
		}
	}

	return loadConfig()
}

func resolveIdentity(ctx context.Context, cfg config.AppConfig, newUsersClient func(config.AppConfig) (usersClient, error)) (authIdentity, error) {
	client, err := newUsersClient(cfg)
	if err != nil {
		return authIdentity{}, apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
	}

	response, err := client.GetUsers2WithResponse(ctx, nil)
	if err != nil {
		return authIdentity{}, apperrors.New(apperrors.KindTransient, "identity lookup failed", err)
	}

	if response.StatusCode() < 200 || response.StatusCode() >= 300 || response.ApplicationjsonCharsetUTF8200 == nil {
		return authIdentity{}, openapi.MapStatusError(response.StatusCode(), response.Body)
	}

	user := response.ApplicationjsonCharsetUTF8200
	return authIdentity{
		Name:        strings.TrimSpace(safeString(user.Name)),
		Slug:        strings.TrimSpace(safeString(user.Slug)),
		DisplayName: strings.TrimSpace(safeString(user.DisplayName)),
		Email:       strings.TrimSpace(safeString(user.EmailAddress)),
		ID:          int64(safeInt32(user.Id)),
		Type:        strings.TrimSpace(safeStringFromEnum(user.Type)),
		Active:      safeBool(user.Active),
	}, nil
}

func identityHumanSummary(identity authIdentity) string {
	parts := make([]string, 0, 6)
	if identity.DisplayName != "" {
		parts = append(parts, identity.DisplayName)
	} else if identity.Name != "" {
		parts = append(parts, identity.Name)
	} else if identity.Slug != "" {
		parts = append(parts, identity.Slug)
	} else {
		parts = append(parts, "unknown")
	}

	if identity.Name != "" {
		parts = append(parts, "name="+identity.Name)
	}
	if identity.Slug != "" {
		parts = append(parts, "slug="+identity.Slug)
	}
	if identity.Email != "" {
		parts = append(parts, "email="+identity.Email)
	}
	if identity.Type != "" {
		parts = append(parts, "type="+identity.Type)
	}
	if identity.ID > 0 {
		parts = append(parts, fmt.Sprintf("id=%d", identity.ID))
	}

	return strings.Join(parts, ", ")
}

func safeString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func safeInt32(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

func safeBool(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

func safeStringFromEnum(value *openapigenerated.RestApplicationUserType) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

// personalAccessTokenURL returns the Bitbucket URL for managing personal access tokens.
// When userSlug is non-empty it returns the per-user URL (.../users/<slug>/manage);
// otherwise it returns the generic manage URL.
func personalAccessTokenURL(host string, userSlug string) (string, error) {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return "", apperrors.New(apperrors.KindValidation, "bitbucket host is required (set --host or BITBUCKET_URL)", nil)
	}

	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return "", apperrors.New(apperrors.KindValidation, "bitbucket host URL is invalid", err)
	}

	slug := strings.TrimSpace(userSlug)
	if slug != "" {
		parsed.Path = path.Join(parsed.Path, "/plugins/servlet/access-tokens/users/"+url.PathEscape(slug)+"/manage")
	} else {
		parsed.Path = path.Join(parsed.Path, "/plugins/servlet/access-tokens/manage")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed.String(), nil
}
