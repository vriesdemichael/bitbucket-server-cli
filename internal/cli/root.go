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
	"github.com/vriesdemichael/bitbucket-server-cli/internal/openapi"
	diffservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/diff"
	reposettings "github.com/vriesdemichael/bitbucket-server-cli/internal/services/reposettings"
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
	rootCmd.AddCommand(newDiffCommand(options))
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

	repoCmd.AddCommand(newRepoSettingsCommand(options))

	return repoCmd
}

func newRepoSettingsCommand(options *rootOptions) *cobra.Command {
	var repositorySelector string

	settingsCmd := &cobra.Command{
		Use:   "settings",
		Short: "Repository settings commands",
	}
	settingsCmd.PersistentFlags().StringVar(&repositorySelector, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")

	securityCmd := &cobra.Command{Use: "security", Short: "Security settings"}
	permissionsCmd := &cobra.Command{Use: "permissions", Short: "Repository permissions"}
	permissionsUsersCmd := &cobra.Command{Use: "users", Short: "User permissions"}
	var permissionsLimit int
	permissionsUsersListCmd := &cobra.Command{
		Use:   "list",
		Short: "List users with repository permissions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := reposettings.NewService(client)
			users, err := service.ListRepositoryPermissionUsers(cmd.Context(), repo, permissionsLimit)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"users": users})
			}
			if len(users) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No users with repository permissions found")
				return nil
			}
			for _, user := range users {
				display := user.Display
				if strings.TrimSpace(display) == "" {
					display = user.Name
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", display, user.Permission)
			}

			return nil
		},
	}
	permissionsUsersListCmd.Flags().IntVar(&permissionsLimit, "limit", 100, "Page size for listing permission users")
	permissionsUsersGrantCmd := &cobra.Command{
		Use:   "grant <username> <permission>",
		Short: "Grant a repository permission to a user",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := reposettings.NewService(client)
			if err := service.GrantRepositoryUserPermission(cmd.Context(), repo, args[0], args[1]); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "username": args[0], "permission": strings.ToUpper(args[1])})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Granted %s to %s\n", strings.ToUpper(args[1]), args[0])
			return nil
		},
	}
	permissionsUsersCmd.AddCommand(permissionsUsersListCmd)
	permissionsUsersCmd.AddCommand(permissionsUsersGrantCmd)
	permissionsCmd.AddCommand(permissionsUsersCmd)
	securityCmd.AddCommand(permissionsCmd)
	settingsCmd.AddCommand(securityCmd)

	workflowCmd := &cobra.Command{Use: "workflow", Short: "Workflow settings"}
	webhooksCmd := &cobra.Command{Use: "webhooks", Short: "Repository webhooks"}
	webhooksListCmd := &cobra.Command{
		Use:   "list",
		Short: "List repository webhooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := reposettings.NewService(client)
			webhooks, err := service.ListRepositoryWebhooks(cmd.Context(), repo)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"webhooks": webhooks.Payload})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Webhooks configured: %d\n", webhooks.Count)
			return nil
		},
	}
	var webhookEvents []string
	var webhookActive bool
	webhooksCreateCmd := &cobra.Command{
		Use:   "create <name> <url>",
		Short: "Create a repository webhook",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := reposettings.NewService(client)
			payload, err := service.CreateRepositoryWebhook(cmd.Context(), repo, reposettings.WebhookCreateInput{
				Name:   args[0],
				URL:    args[1],
				Events: webhookEvents,
				Active: webhookActive,
			})
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "webhook": payload})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Webhook created: %s\n", args[0])
			return nil
		},
	}
	webhooksCreateCmd.Flags().StringSliceVar(&webhookEvents, "event", []string{"repo:refs_changed"}, "Webhook event(s) to subscribe to")
	webhooksCreateCmd.Flags().BoolVar(&webhookActive, "active", true, "Whether the webhook is active")
	webhooksDeleteCmd := &cobra.Command{
		Use:   "delete <webhook-id>",
		Short: "Delete a repository webhook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := reposettings.NewService(client)
			if err := service.DeleteRepositoryWebhook(cmd.Context(), repo, args[0]); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "webhook_id": args[0]})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Webhook deleted: %s\n", args[0])
			return nil
		},
	}
	webhooksCmd.AddCommand(webhooksListCmd)
	webhooksCmd.AddCommand(webhooksCreateCmd)
	webhooksCmd.AddCommand(webhooksDeleteCmd)
	workflowCmd.AddCommand(webhooksCmd)
	settingsCmd.AddCommand(workflowCmd)

	pullRequestsCmd := &cobra.Command{Use: "pull-requests", Short: "Pull request settings"}
	pullRequestsGetCmd := &cobra.Command{
		Use:   "get",
		Short: "Get repository pull-request settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := reposettings.NewService(client)
			settings, err := service.GetRepositoryPullRequestSettings(cmd.Context(), repo)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"pull_request_settings": settings})
			}

			requiredTasks := false
			if value, ok := settings["requiredAllTasksComplete"].(bool); ok {
				requiredTasks = value
			}
			requiredApprovals := "disabled"
			if section, ok := settings["requiredApprovers"].(map[string]any); ok {
				enabled, _ := section["enabled"].(bool)
				if enabled {
					switch count := section["count"].(type) {
					case string:
						requiredApprovals = count
					case float64:
						requiredApprovals = fmt.Sprintf("%.0f", count)
					default:
						requiredApprovals = "enabled"
					}
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Required tasks complete: %t\n", requiredTasks)
			fmt.Fprintf(cmd.OutOrStdout(), "Required approvers: %s\n", requiredApprovals)
			return nil
		},
	}
	var requiredAllTasksComplete bool
	var requiredApproversCount int
	pullRequestsUpdateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update repository pull-request settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := reposettings.NewService(client)
			settings, err := service.UpdateRepositoryPullRequestRequiredAllTasks(cmd.Context(), repo, requiredAllTasksComplete)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "pull_request_settings": settings})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated pull-request settings: requiredAllTasksComplete=%t\n", requiredAllTasksComplete)
			return nil
		},
	}
	pullRequestsUpdateCmd.Flags().BoolVar(&requiredAllTasksComplete, "required-all-tasks-complete", false, "Require all pull-request tasks to be completed before merge")
	pullRequestsUpdateApproversCmd := &cobra.Command{
		Use:   "update-approvers",
		Short: "Update required approvers count",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := reposettings.NewService(client)
			settings, err := service.UpdateRepositoryPullRequestRequiredApproversCount(cmd.Context(), repo, requiredApproversCount)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "pull_request_settings": settings})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated pull-request settings: requiredApprovers=%d\n", requiredApproversCount)
			return nil
		},
	}
	pullRequestsUpdateApproversCmd.Flags().IntVar(&requiredApproversCount, "count", 2, "Required approvers count (0 disables check)")
	pullRequestsCmd.AddCommand(pullRequestsGetCmd)
	pullRequestsCmd.AddCommand(pullRequestsUpdateCmd)
	pullRequestsCmd.AddCommand(pullRequestsUpdateApproversCmd)
	settingsCmd.AddCommand(pullRequestsCmd)

	return settingsCmd
}

func newDiffCommand(options *rootOptions) *cobra.Command {
	var repositorySelector string

	diffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff and patch commands",
	}

	diffCmd.PersistentFlags().StringVar(&repositorySelector, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")

	var refsPath string
	var refsPatch bool
	var refsStat bool
	var refsNameOnly bool

	refsCmd := &cobra.Command{
		Use:   "refs <from> <to>",
		Short: "Diff two refs or commits",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := diffservice.NewService(client)
			outputMode, err := resolveDiffOutputMode(refsPatch, refsStat, refsNameOnly)
			if err != nil {
				return err
			}

			result, err := service.DiffRefs(cmd.Context(), diffservice.DiffRefsInput{
				Repository: repo,
				From:       args[0],
				To:         args[1],
				Path:       refsPath,
				Output:     outputMode,
			})
			if err != nil {
				return err
			}

			return writeDiffResult(cmd.OutOrStdout(), options.JSON, outputMode, result)
		},
	}
	refsCmd.Flags().StringVar(&refsPath, "path", "", "Optional file path for file-scoped diff")
	refsCmd.Flags().BoolVar(&refsPatch, "patch", false, "Output unified patch stream")
	refsCmd.Flags().BoolVar(&refsStat, "stat", false, "Output structured diff stats")
	refsCmd.Flags().BoolVar(&refsNameOnly, "name-only", false, "Output only changed file names")
	diffCmd.AddCommand(refsCmd)

	var prPatch bool
	var prStat bool
	var prNameOnly bool

	prCmd := &cobra.Command{
		Use:   "pr <id>",
		Short: "Diff a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := diffservice.NewService(client)
			outputMode, err := resolveDiffOutputMode(prPatch, prStat, prNameOnly)
			if err != nil {
				return err
			}

			result, err := service.DiffPR(cmd.Context(), diffservice.DiffPRInput{
				Repository:    repo,
				PullRequestID: args[0],
				Output:        outputMode,
			})
			if err != nil {
				return err
			}

			return writeDiffResult(cmd.OutOrStdout(), options.JSON, outputMode, result)
		},
	}
	prCmd.Flags().BoolVar(&prPatch, "patch", false, "Output unified patch stream")
	prCmd.Flags().BoolVar(&prStat, "stat", false, "Output structured diff stats")
	prCmd.Flags().BoolVar(&prNameOnly, "name-only", false, "Output only changed file names")
	diffCmd.AddCommand(prCmd)

	var commitPath string

	commitCmd := &cobra.Command{
		Use:   "commit <sha>",
		Short: "Diff a commit against its parent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := diffservice.NewService(client)
			result, err := service.DiffCommit(cmd.Context(), diffservice.DiffCommitInput{
				Repository: repo,
				CommitID:   args[0],
				Path:       commitPath,
			})
			if err != nil {
				return err
			}

			return writeDiffResult(cmd.OutOrStdout(), options.JSON, diffservice.OutputKindRaw, result)
		},
	}
	commitCmd.Flags().StringVar(&commitPath, "path", "", "Optional file path for file-scoped diff")
	diffCmd.AddCommand(commitCmd)

	return diffCmd
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

func resolveRepositoryReference(selector string, cfg config.AppConfig) (diffservice.RepositoryRef, error) {
	repo, err := resolveRepositorySelector(selector, cfg)
	if err != nil {
		return diffservice.RepositoryRef{}, err
	}

	return diffservice.RepositoryRef{ProjectKey: repo.ProjectKey, Slug: repo.Slug}, nil
}

type repositorySelector struct {
	ProjectKey string
	Slug       string
}

func resolveRepositorySelector(selector string, cfg config.AppConfig) (repositorySelector, error) {
	trimmed := strings.TrimSpace(selector)
	if trimmed == "" {
		repoSlug := strings.TrimSpace(os.Getenv("BITBUCKET_REPO_SLUG"))
		if strings.TrimSpace(cfg.ProjectKey) == "" || repoSlug == "" {
			return repositorySelector{}, apperrors.New(
				apperrors.KindValidation,
				"repository is required (use --repo PROJECT/slug or set BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)",
				nil,
			)
		}

		return repositorySelector{ProjectKey: cfg.ProjectKey, Slug: repoSlug}, nil
	}

	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return repositorySelector{}, apperrors.New(apperrors.KindValidation, "--repo must be in PROJECT/slug format", nil)
	}

	return repositorySelector{ProjectKey: strings.TrimSpace(parts[0]), Slug: strings.TrimSpace(parts[1])}, nil
}

func resolveRepositorySettingsReference(selector string, cfg config.AppConfig) (reposettings.RepositoryRef, error) {
	repo, err := resolveRepositorySelector(selector, cfg)
	if err != nil {
		return reposettings.RepositoryRef{}, err
	}

	return reposettings.RepositoryRef{ProjectKey: repo.ProjectKey, Slug: repo.Slug}, nil
}

func resolveDiffOutputMode(patch, stat, nameOnly bool) (diffservice.OutputKind, error) {
	selected := 0
	if patch {
		selected++
	}
	if stat {
		selected++
	}
	if nameOnly {
		selected++
	}
	if selected > 1 {
		return "", apperrors.New(apperrors.KindValidation, "choose only one output mode: --patch, --stat, or --name-only", nil)
	}

	if patch {
		return diffservice.OutputKindPatch, nil
	}
	if stat {
		return diffservice.OutputKindStat, nil
	}
	if nameOnly {
		return diffservice.OutputKindNameOnly, nil
	}

	return diffservice.OutputKindRaw, nil
}

func writeDiffResult(writer io.Writer, asJSON bool, mode diffservice.OutputKind, result diffservice.Result) error {
	if asJSON {
		switch mode {
		case diffservice.OutputKindNameOnly:
			return writeJSON(writer, map[string]any{"names": result.Names})
		case diffservice.OutputKindStat:
			return writeJSON(writer, map[string]any{"stats": result.Stats})
		default:
			return writeJSON(writer, map[string]any{"patch": result.Patch})
		}
	}

	switch mode {
	case diffservice.OutputKindNameOnly:
		for _, name := range result.Names {
			fmt.Fprintln(writer, name)
		}
		return nil
	case diffservice.OutputKindStat:
		return writeJSON(writer, result.Stats)
	default:
		fmt.Fprint(writer, result.Patch)
		if result.Patch != "" && !strings.HasSuffix(result.Patch, "\n") {
			fmt.Fprintln(writer)
		}
		return nil
	}
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
