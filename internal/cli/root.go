package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/openapi"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
	commentservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/comment"
	diffservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/diff"
	qualityservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/quality"
	reposettings "github.com/vriesdemichael/bitbucket-server-cli/internal/services/reposettings"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/services/repository"
	tagservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/tag"
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
	rootCmd.AddCommand(newTagCommand(options))
	rootCmd.AddCommand(newDiffCommand(options))
	rootCmd.AddCommand(newBuildCommand(options))
	rootCmd.AddCommand(newInsightsCommand(options))
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
	repoCmd.AddCommand(newRepoCommentCommand(options))

	return repoCmd
}

func newRepoCommentCommand(options *rootOptions) *cobra.Command {
	var repositorySelector string
	var commitID string
	var pullRequestID string

	commentCmd := &cobra.Command{
		Use:   "comment",
		Short: "Comment commands for commits and pull requests",
	}

	commentCmd.PersistentFlags().StringVar(&repositorySelector, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	commentCmd.PersistentFlags().StringVar(&commitID, "commit", "", "Commit ID context")
	commentCmd.PersistentFlags().StringVar(&pullRequestID, "pr", "", "Pull request ID context")

	var listLimit int
	var listPath string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List comments",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			target, err := resolveCommentTarget(repositorySelector, commitID, pullRequestID, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := commentservice.NewService(client)
			comments, err := service.List(cmd.Context(), target, listPath, listLimit)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"context": target.Context(), "comments": comments})
			}

			if len(comments) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No comments found")
				return nil
			}

			for _, comment := range comments {
				fmt.Fprintln(cmd.OutOrStdout(), formatCommentSummary(comment))
			}

			return nil
		},
	}
	listCmd.Flags().StringVar(&listPath, "path", "", "File path for comment listing scope")
	listCmd.Flags().IntVar(&listLimit, "limit", 25, "Page size for Bitbucket comment list operations")
	_ = listCmd.MarkFlagRequired("path")
	commentCmd.AddCommand(listCmd)

	var createText string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a comment",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			target, err := resolveCommentTarget(repositorySelector, commitID, pullRequestID, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := commentservice.NewService(client)
			created, err := service.Create(cmd.Context(), target, createText)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"context": target.Context(), "comment": created})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created comment %s\n", commentIDString(created))
			return nil
		},
	}
	createCmd.Flags().StringVar(&createText, "text", "", "Comment text")
	_ = createCmd.MarkFlagRequired("text")
	commentCmd.AddCommand(createCmd)

	var updateCommentID string
	var updateText string
	var updateVersion int32
	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a comment",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			target, err := resolveCommentTarget(repositorySelector, commitID, pullRequestID, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := commentservice.NewService(client)

			var version *int32
			if cmd.Flags().Changed("version") {
				version = &updateVersion
			}

			updated, err := service.Update(cmd.Context(), target, updateCommentID, updateText, version)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"context": target.Context(), "comment": updated})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated comment %s\n", commentIDString(updated))
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateCommentID, "id", "", "Comment ID")
	updateCmd.Flags().StringVar(&updateText, "text", "", "Comment text")
	updateCmd.Flags().Int32Var(&updateVersion, "version", 0, "Expected comment version")
	_ = updateCmd.MarkFlagRequired("id")
	_ = updateCmd.MarkFlagRequired("text")
	commentCmd.AddCommand(updateCmd)

	var deleteCommentID string
	var deleteVersion int32
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a comment",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			target, err := resolveCommentTarget(repositorySelector, commitID, pullRequestID, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := commentservice.NewService(client)

			var version *int32
			if cmd.Flags().Changed("version") {
				version = &deleteVersion
			}

			resolvedVersion, err := service.Delete(cmd.Context(), target, deleteCommentID, version)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"context": target.Context(),
					"deleted": map[string]any{"id": deleteCommentID, "version": resolvedVersion},
				})
			}

			if resolvedVersion == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Deleted comment %s\n", strings.TrimSpace(deleteCommentID))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted comment %s (version=%d)\n", strings.TrimSpace(deleteCommentID), *resolvedVersion)
			return nil
		},
	}
	deleteCmd.Flags().StringVar(&deleteCommentID, "id", "", "Comment ID")
	deleteCmd.Flags().Int32Var(&deleteVersion, "version", 0, "Expected comment version")
	_ = deleteCmd.MarkFlagRequired("id")
	commentCmd.AddCommand(deleteCmd)

	return commentCmd
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

func newTagCommand(options *rootOptions) *cobra.Command {
	var repositorySelector string
	var limit int
	var orderBy string
	var filterText string

	tagCmd := &cobra.Command{
		Use:   "tag",
		Short: "Repository tag lifecycle commands",
	}

	tagCmd.PersistentFlags().StringVar(&repositorySelector, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	tagCmd.PersistentFlags().IntVar(&limit, "limit", 25, "Page size for list operations")
	tagCmd.PersistentFlags().StringVar(&orderBy, "order-by", "", "Tag ordering: ALPHABETICAL or MODIFICATION")
	tagCmd.PersistentFlags().StringVar(&filterText, "filter", "", "Filter text for tag names")

	tagCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List repository tags",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveTagRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := tagservice.NewService(client)
			tags, err := service.List(cmd.Context(), repo, tagservice.ListOptions{Limit: limit, OrderBy: orderBy, FilterText: filterText})
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), tags)
			}

			if len(tags) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No tags found")
				return nil
			}

			for _, tag := range tags {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", safeString(tag.DisplayId), safeStringFromTagType(tag.Type), safeString(tag.LatestCommit))
			}

			return nil
		},
	})

	var startPoint string
	var message string
	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create repository tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveTagRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := tagservice.NewService(client)
			createdTag, err := service.Create(cmd.Context(), repo, args[0], startPoint, message)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), createdTag)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created tag %s (%s)\n", safeString(createdTag.DisplayId), safeString(createdTag.LatestCommit))
			return nil
		},
	}
	createCmd.Flags().StringVar(&startPoint, "start-point", "", "Commit ID or ref to tag")
	createCmd.Flags().StringVar(&message, "message", "", "Optional annotated tag message")
	_ = createCmd.MarkFlagRequired("start-point")
	tagCmd.AddCommand(createCmd)

	tagCmd.AddCommand(&cobra.Command{
		Use:   "view <name>",
		Short: "View repository tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveTagRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := tagservice.NewService(client)
			tag, err := service.Get(cmd.Context(), repo, args[0])
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), tag)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Tag: %s\n", safeString(tag.DisplayId))
			fmt.Fprintf(cmd.OutOrStdout(), "Type: %s\n", safeStringFromTagType(tag.Type))
			fmt.Fprintf(cmd.OutOrStdout(), "Commit: %s\n", safeString(tag.LatestCommit))
			return nil
		},
	})

	tagCmd.AddCommand(&cobra.Command{
		Use:   "delete <name>",
		Short: "Delete repository tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveTagRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := tagservice.NewService(client)
			if err := service.Delete(cmd.Context(), repo, args[0]); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"status": "ok", "tag": args[0]})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted tag %s\n", args[0])
			return nil
		},
	})

	return tagCmd
}

func newBuildCommand(options *rootOptions) *cobra.Command {
	var repositorySelector string

	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Build status and required merge-check commands",
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Build status commands by commit",
	}

	var setKey string
	var setState string
	var setURL string
	var setName string
	var setDescription string
	var setRef string
	var setParent string
	var setBuildNumber string
	var setDuration int64
	setCmd := &cobra.Command{
		Use:   "set <commit>",
		Short: "Set build status for a commit",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			if err := service.SetBuildStatus(cmd.Context(), args[0], qualityservice.BuildStatusSetInput{
				Key:         setKey,
				State:       setState,
				URL:         setURL,
				Name:        setName,
				Description: setDescription,
				Ref:         setRef,
				Parent:      setParent,
				BuildNumber: setBuildNumber,
				DurationMS:  setDuration,
			}); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"status": "ok", "commit": args[0], "key": setKey})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Build status %s set on %s\n", setKey, args[0])
			return nil
		},
	}
	setCmd.Flags().StringVar(&setKey, "key", "", "Build status key")
	setCmd.Flags().StringVar(&setState, "state", "", "Build state (SUCCESSFUL, FAILED, INPROGRESS, UNKNOWN)")
	setCmd.Flags().StringVar(&setURL, "url", "", "Build URL")
	setCmd.Flags().StringVar(&setName, "name", "", "Build display name")
	setCmd.Flags().StringVar(&setDescription, "description", "", "Build description")
	setCmd.Flags().StringVar(&setRef, "ref", "", "Build ref")
	setCmd.Flags().StringVar(&setParent, "parent", "", "Build parent key")
	setCmd.Flags().StringVar(&setBuildNumber, "build-number", "", "Build number")
	setCmd.Flags().Int64Var(&setDuration, "duration-ms", 0, "Duration in milliseconds")
	_ = setCmd.MarkFlagRequired("key")
	_ = setCmd.MarkFlagRequired("state")
	_ = setCmd.MarkFlagRequired("url")
	statusCmd.AddCommand(setCmd)

	var getLimit int
	var getOrderBy string
	statusCmd.AddCommand(&cobra.Command{
		Use:   "get <commit>",
		Short: "Get build statuses for a commit",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			statuses, err := service.GetBuildStatuses(cmd.Context(), args[0], getLimit, getOrderBy)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), statuses)
			}

			if len(statuses) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No build statuses found")
				return nil
			}

			for _, status := range statuses {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", safeString(status.Key), safeStringFromBuildState(status.State), safeString(status.Url))
			}

			return nil
		},
	})
	statusCmd.PersistentFlags().IntVar(&getLimit, "limit", 25, "Page size for list operations")
	statusCmd.PersistentFlags().StringVar(&getOrderBy, "order-by", "", "Order by NEWEST, OLDEST, or STATUS")

	var includeUnique bool
	statusCmd.AddCommand(&cobra.Command{
		Use:   "stats <commit>",
		Short: "Get build status summary counts for a commit",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			stats, err := service.GetBuildStatusStats(cmd.Context(), args[0], includeUnique)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), stats)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successful: %d\n", safeInt32(stats.Successful))
			fmt.Fprintf(cmd.OutOrStdout(), "Failed: %d\n", safeInt32(stats.Failed))
			fmt.Fprintf(cmd.OutOrStdout(), "In Progress: %d\n", safeInt32(stats.InProgress))
			fmt.Fprintf(cmd.OutOrStdout(), "Unknown: %d\n", safeInt32(stats.Unknown))
			fmt.Fprintf(cmd.OutOrStdout(), "Cancelled: %d\n", safeInt32(stats.Cancelled))
			return nil
		},
	})
	statusCmd.PersistentFlags().BoolVar(&includeUnique, "include-unique", false, "Include unique result details when available")

	requiredCmd := &cobra.Command{
		Use:   "required",
		Short: "Required build merge-check management",
	}
	requiredCmd.PersistentFlags().StringVar(&repositorySelector, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")

	var requiredLimit int
	requiredCmd.PersistentFlags().IntVar(&requiredLimit, "limit", 25, "Page size for list operations")

	requiredCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List required build merge checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveQualityRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			checks, err := service.ListRequiredBuildChecks(cmd.Context(), repo, requiredLimit)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), checks)
			}

			if len(checks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No required build merge checks found")
				return nil
			}

			for _, check := range checks {
				fmt.Fprintf(cmd.OutOrStdout(), "id=%d buildParentKeys=%v\n", safeInt64(check.Id), safeStringSlice(check.BuildParentKeys))
			}

			return nil
		},
	})

	var createBody string
	createRequiredCmd := &cobra.Command{
		Use:   "create",
		Short: "Create required build merge check",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveQualityRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			payload := map[string]any{}
			if err := json.Unmarshal([]byte(createBody), &payload); err != nil {
				return apperrors.New(apperrors.KindValidation, "invalid JSON for --body", err)
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			created, err := service.CreateRequiredBuildCheck(cmd.Context(), repo, payload)
			if err != nil {
				return err
			}

			return writeJSON(cmd.OutOrStdout(), created)
		},
	}
	createRequiredCmd.Flags().StringVar(&createBody, "body", "", "Raw JSON payload for required build merge check")
	_ = createRequiredCmd.MarkFlagRequired("body")
	requiredCmd.AddCommand(createRequiredCmd)

	var updateBody string
	updateRequiredCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update required build merge check",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveQualityRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return apperrors.New(apperrors.KindValidation, "merge check id must be a valid integer", err)
			}

			payload := map[string]any{}
			if err := json.Unmarshal([]byte(updateBody), &payload); err != nil {
				return apperrors.New(apperrors.KindValidation, "invalid JSON for --body", err)
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			updated, err := service.UpdateRequiredBuildCheck(cmd.Context(), repo, id, payload)
			if err != nil {
				return err
			}

			return writeJSON(cmd.OutOrStdout(), updated)
		},
	}
	updateRequiredCmd.Flags().StringVar(&updateBody, "body", "", "Raw JSON payload for required build merge check")
	_ = updateRequiredCmd.MarkFlagRequired("body")
	requiredCmd.AddCommand(updateRequiredCmd)

	requiredCmd.AddCommand(&cobra.Command{
		Use:   "delete <id>",
		Short: "Delete required build merge check",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveQualityRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return apperrors.New(apperrors.KindValidation, "merge check id must be a valid integer", err)
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			if err := service.DeleteRequiredBuildCheck(cmd.Context(), repo, id); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "id": id})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted required build merge check %d\n", id)
			return nil
		},
	})

	buildCmd.AddCommand(statusCmd)
	buildCmd.AddCommand(requiredCmd)

	return buildCmd
}

func newInsightsCommand(options *rootOptions) *cobra.Command {
	var repositorySelector string
	var reportLimit int

	insightsCmd := &cobra.Command{
		Use:   "insights",
		Short: "Code Insights report and annotation commands",
	}
	insightsCmd.PersistentFlags().StringVar(&repositorySelector, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")

	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Code Insights report commands",
	}
	reportCmd.PersistentFlags().IntVar(&reportLimit, "limit", 25, "Page size for list operations")

	var reportBody string
	setReportCmd := &cobra.Command{
		Use:   "set <commit> <key>",
		Short: "Create or update a Code Insights report",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveQualityRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			request := openapigenerated.SetACodeInsightsReportJSONRequestBody{}
			if err := json.Unmarshal([]byte(reportBody), &request); err != nil {
				return apperrors.New(apperrors.KindValidation, "invalid JSON for --body", err)
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			report, err := service.SetReport(cmd.Context(), repo, args[0], args[1], request)
			if err != nil {
				return err
			}

			return writeJSON(cmd.OutOrStdout(), report)
		},
	}
	setReportCmd.Flags().StringVar(&reportBody, "body", "", "Raw JSON payload for Code Insights report")
	_ = setReportCmd.MarkFlagRequired("body")
	reportCmd.AddCommand(setReportCmd)

	reportCmd.AddCommand(&cobra.Command{
		Use:   "get <commit> <key>",
		Short: "Get a Code Insights report",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveQualityRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			report, err := service.GetReport(cmd.Context(), repo, args[0], args[1])
			if err != nil {
				return err
			}

			return writeJSON(cmd.OutOrStdout(), report)
		},
	})

	reportCmd.AddCommand(&cobra.Command{
		Use:   "delete <commit> <key>",
		Short: "Delete a Code Insights report",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveQualityRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			if err := service.DeleteReport(cmd.Context(), repo, args[0], args[1]); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "commit": args[0], "key": args[1]})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted report %s for commit %s\n", args[1], args[0])
			return nil
		},
	})

	reportCmd.AddCommand(&cobra.Command{
		Use:   "list <commit>",
		Short: "List Code Insights reports for a commit",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveQualityRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			reports, err := service.ListReports(cmd.Context(), repo, args[0], reportLimit)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), reports)
			}

			if len(reports) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No reports found")
				return nil
			}

			for _, report := range reports {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", safeString(report.Key), safeString(report.Title), safeStringFromInsightResult(report.Result))
			}

			return nil
		},
	})

	annotationCmd := &cobra.Command{
		Use:   "annotation",
		Short: "Code Insights annotation commands",
	}

	var annotationBody string
	addAnnotationCmd := &cobra.Command{
		Use:   "add <commit> <key>",
		Short: "Add annotations to a Code Insights report",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveQualityRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			annotations := make([]openapigenerated.RestSingleAddInsightAnnotationRequest, 0)
			if err := json.Unmarshal([]byte(annotationBody), &annotations); err != nil {
				return apperrors.New(apperrors.KindValidation, "invalid JSON for --body (expected array of annotations)", err)
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			if err := service.AddAnnotations(cmd.Context(), repo, args[0], args[1], annotations); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "count": len(annotations)})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added %d annotations to report %s\n", len(annotations), args[1])
			return nil
		},
	}
	addAnnotationCmd.Flags().StringVar(&annotationBody, "body", "", "Raw JSON array payload for annotations")
	_ = addAnnotationCmd.MarkFlagRequired("body")
	annotationCmd.AddCommand(addAnnotationCmd)

	annotationCmd.AddCommand(&cobra.Command{
		Use:   "list <commit> <key>",
		Short: "List annotations for a Code Insights report",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveQualityRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			annotations, err := service.ListAnnotations(cmd.Context(), repo, args[0], args[1])
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), annotations)
			}

			if len(annotations) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No annotations found")
				return nil
			}

			for _, annotation := range annotations {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", safeString(annotation.ExternalId), safeString(annotation.Severity), safeString(annotation.Message))
			}

			return nil
		},
	})

	var externalID string
	deleteAnnotationCmd := &cobra.Command{
		Use:   "delete <commit> <key>",
		Short: "Delete annotation(s) by external id for a report",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}

			repo, err := resolveQualityRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			client, err := openapi.NewClientWithResponsesFromConfig(cfg)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "failed to initialize API client", err)
			}

			service := qualityservice.NewService(client)
			if err := service.DeleteAnnotations(cmd.Context(), repo, args[0], args[1], externalID); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "external_id": externalID})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted annotations for external id %s\n", externalID)
			return nil
		},
	}
	deleteAnnotationCmd.Flags().StringVar(&externalID, "external-id", "", "External annotation ID to delete")
	_ = deleteAnnotationCmd.MarkFlagRequired("external-id")
	annotationCmd.AddCommand(deleteAnnotationCmd)

	insightsCmd.AddCommand(reportCmd)
	insightsCmd.AddCommand(annotationCmd)

	return insightsCmd
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

func resolveTagRepositoryReference(selector string, cfg config.AppConfig) (tagservice.RepositoryRef, error) {
	repo, err := resolveRepositorySelector(selector, cfg)
	if err != nil {
		return tagservice.RepositoryRef{}, err
	}

	return tagservice.RepositoryRef{ProjectKey: repo.ProjectKey, Slug: repo.Slug}, nil
}

func resolveQualityRepositoryReference(selector string, cfg config.AppConfig) (qualityservice.RepositoryRef, error) {
	repo, err := resolveRepositorySelector(selector, cfg)
	if err != nil {
		return qualityservice.RepositoryRef{}, err
	}

	return qualityservice.RepositoryRef{ProjectKey: repo.ProjectKey, Slug: repo.Slug}, nil
}

func resolveCommentTarget(selector string, commitID string, pullRequestID string, cfg config.AppConfig) (commentservice.Target, error) {
	repo, err := resolveRepositorySelector(selector, cfg)
	if err != nil {
		return commentservice.Target{}, err
	}

	trimmedCommitID := strings.TrimSpace(commitID)
	trimmedPullRequestID := strings.TrimSpace(pullRequestID)
	hasCommit := trimmedCommitID != ""
	hasPullRequest := trimmedPullRequestID != ""

	if hasCommit == hasPullRequest {
		return commentservice.Target{}, apperrors.New(apperrors.KindValidation, "exactly one of --commit or --pr is required", nil)
	}

	return commentservice.Target{
		Repository:    commentservice.RepositoryRef{ProjectKey: repo.ProjectKey, Slug: repo.Slug},
		CommitID:      trimmedCommitID,
		PullRequestID: trimmedPullRequestID,
	}, nil
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

func commentIDString(comment openapigenerated.RestComment) string {
	if comment.Id == nil {
		return "unknown"
	}

	return strconv.FormatInt(*comment.Id, 10)
}

func formatCommentSummary(comment openapigenerated.RestComment) string {
	text := ""
	if comment.Text != nil {
		text = strings.TrimSpace(*comment.Text)
	}
	if text == "" {
		text = "<empty>"
	}

	version := "?"
	if comment.Version != nil {
		version = strconv.Itoa(int(*comment.Version))
	}

	return fmt.Sprintf("[%s v%s] %s", commentIDString(comment), version, text)
}

func writeJSON(writer io.Writer, payload any) error {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "failed to encode JSON output", err)
	}

	fmt.Fprintln(writer, string(encoded))
	return nil
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

func safeInt64(value *int64) int64 {
	if value == nil {
		return 0
	}

	return *value
}

func safeStringSlice(values *[]string) []string {
	if values == nil {
		return []string{}
	}

	return *values
}

func safeStringFromTagType(tagType *openapigenerated.RestTagType) string {
	if tagType == nil {
		return ""
	}

	return string(*tagType)
}

func safeStringFromBuildState(state *openapigenerated.RestBuildStatusState) string {
	if state == nil {
		return ""
	}

	return string(*state)
}

func safeStringFromInsightResult(result *openapigenerated.RestInsightReportResult) string {
	if result == nil {
		return ""
	}

	return string(*result)
}
