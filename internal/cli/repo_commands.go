package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	commentservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/comment"
	reposettings "github.com/vriesdemichael/bitbucket-server-cli/internal/services/reposettings"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/services/repository"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/transport/httpclient"
)

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
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			client := httpclient.NewFromConfig(cfg)
			service := repository.NewService(client)

			repos, err := service.List(cmd.Context(), repository.ListOptions{Limit: limit})
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
	repoCmd.AddCommand(newRepoBrowseCommand(options))
	repoCmd.AddCommand(newRepoAdminCommand(options))

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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			target, err := resolveCommentTarget(repositorySelector, commitID, pullRequestID, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			target, err := resolveCommentTarget(repositorySelector, commitID, pullRequestID, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			target, err := resolveCommentTarget(repositorySelector, commitID, pullRequestID, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			target, err := resolveCommentTarget(repositorySelector, commitID, pullRequestID, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
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
	permissionsUsersRevokeCmd := &cobra.Command{
		Use:   "revoke <username>",
		Short: "Revoke a repository permission from a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			service := reposettings.NewService(client)
			if err := service.RevokeRepositoryUserPermission(cmd.Context(), repo, args[0]); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "username": args[0]})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Revoked permissions for %s\n", args[0])
			return nil
		},
	}

	permissionsGroupsCmd := &cobra.Command{Use: "groups", Short: "Group permissions"}
	permissionsGroupsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List groups with repository permissions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			service := reposettings.NewService(client)
			groups, err := service.ListRepositoryPermissionGroups(cmd.Context(), repo, permissionsLimit)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"groups": groups})
			}
			if len(groups) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No groups with repository permissions found")
				return nil
			}
			for _, group := range groups {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", group.Name, group.Permission)
			}

			return nil
		},
	}
	permissionsGroupsListCmd.Flags().IntVar(&permissionsLimit, "limit", 100, "Page size for listing permission groups")

	permissionsGroupsGrantCmd := &cobra.Command{
		Use:   "grant <group> <permission>",
		Short: "Grant a repository permission to a group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			service := reposettings.NewService(client)
			if err := service.GrantRepositoryGroupPermission(cmd.Context(), repo, args[0], args[1]); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "group": args[0], "permission": strings.ToUpper(args[1])})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Granted %s to group %s\n", strings.ToUpper(args[1]), args[0])
			return nil
		},
	}

	permissionsGroupsRevokeCmd := &cobra.Command{
		Use:   "revoke <group>",
		Short: "Revoke a repository permission from a group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			service := reposettings.NewService(client)
			if err := service.RevokeRepositoryGroupPermission(cmd.Context(), repo, args[0]); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "group": args[0]})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Revoked permissions for group %s\n", args[0])
			return nil
		},
	}

	permissionsUsersCmd.AddCommand(permissionsUsersListCmd)
	permissionsUsersCmd.AddCommand(permissionsUsersGrantCmd)
	permissionsUsersCmd.AddCommand(permissionsUsersRevokeCmd)
	permissionsCmd.AddCommand(permissionsUsersCmd)

	permissionsGroupsCmd.AddCommand(permissionsGroupsListCmd)
	permissionsGroupsCmd.AddCommand(permissionsGroupsGrantCmd)
	permissionsGroupsCmd.AddCommand(permissionsGroupsRevokeCmd)
	permissionsCmd.AddCommand(permissionsGroupsCmd)

	securityCmd.AddCommand(permissionsCmd)
	settingsCmd.AddCommand(securityCmd)

	workflowCmd := &cobra.Command{Use: "workflow", Short: "Workflow settings"}
	webhooksCmd := &cobra.Command{Use: "webhooks", Short: "Repository webhooks"}
	webhooksListCmd := &cobra.Command{
		Use:   "list",
		Short: "List repository webhooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
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

			if mergeConfig, ok := settings["mergeConfig"].(map[string]any); ok {
				if strategies, ok := mergeConfig["strategies"].([]any); ok {
					fmt.Fprintf(cmd.OutOrStdout(), "Available merge strategies: %d\n", len(strategies))
					for _, s := range strategies {
						if sm, ok := s.(map[string]any); ok {
							enabled := ""
							if en, ok := sm["enabled"].(bool); ok && en {
								enabled = "*"
							}
							fmt.Fprintf(cmd.OutOrStdout(), "- %s%s (%s)\n", sm["id"], enabled, sm["name"])
						}
					}
				}
			}
			return nil
		},
	}

	mergeChecksCmd := &cobra.Command{
		Use:   "merge-checks",
		Short: "Manage repository merge checks",
	}
	mergeChecksListCmd := &cobra.Command{
		Use:   "list",
		Short: "List configured merge checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			service := reposettings.NewService(client)
			checks, err := service.ListRequiredBuildsMergeChecks(cmd.Context(), repo)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"merge_checks": checks})
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Configured merge checks:")
			if checksMap, ok := checks.(map[string]any); ok {
				if values, ok := checksMap["values"].([]any); ok {
					for _, check := range values {
						fmt.Fprintf(cmd.OutOrStdout(), "- %v\n", check)
					}
					return nil
				}
			}

			if checksArr, ok := checks.([]any); ok {
				for _, check := range checksArr {
					fmt.Fprintf(cmd.OutOrStdout(), "- %v\n", check)
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "- %v\n", checks)
			}
			return nil
		},
	}
	mergeChecksCmd.AddCommand(mergeChecksListCmd)
	pullRequestsCmd.AddCommand(mergeChecksCmd)
	var requiredAllTasksComplete bool
	var requiredApproversCount int
	pullRequestsUpdateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update repository pull-request settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
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
	pullRequestsSetStrategyCmd := &cobra.Command{
		Use:   "set-strategy <strategy-id>",
		Short: "Set default merge strategy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositorySettingsReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			service := reposettings.NewService(client)
			mergeStrategyID := args[0]

			settings := map[string]any{
				"mergeConfig": map[string]any{
					"defaultStrategy": map[string]any{
						"id": mergeStrategyID,
					},
				},
			}

			updated, err := service.UpdateRepositoryPullRequestSettings(cmd.Context(), repo, settings)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "pull_request_settings": updated})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated default merge strategy to %s\n", mergeStrategyID)
			return nil
		},
	}
	pullRequestsCmd.AddCommand(pullRequestsSetStrategyCmd)

	pullRequestsCmd.AddCommand(pullRequestsGetCmd)
	pullRequestsCmd.AddCommand(pullRequestsUpdateCmd)
	pullRequestsCmd.AddCommand(pullRequestsUpdateApproversCmd)
	settingsCmd.AddCommand(pullRequestsCmd)

	return settingsCmd
}
