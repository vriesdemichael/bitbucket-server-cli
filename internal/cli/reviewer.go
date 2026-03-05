package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
	reviewerservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/reviewer"
)

func newReviewerCommand(options *rootOptions) *cobra.Command {
	var projectKey string
	var repositorySelector string

	reviewerCmd := &cobra.Command{
		Use:   "reviewer",
		Short: "Manage default reviewers",
	}

	reviewerCmd.PersistentFlags().StringVar(&projectKey, "project", "", "Project key")
	reviewerCmd.PersistentFlags().StringVar(&repositorySelector, "repo", "", "Repository as PROJECT/slug")

	conditionCmd := &cobra.Command{
		Use:   "condition",
		Short: "Manage default reviewer conditions",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List default reviewer conditions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := reviewerservice.NewService(client)

			if repositorySelector != "" {
				repo, err := resolveRepositoryReference(repositorySelector, cfg)
				if err != nil {
					return err
				}
				conditions, err := service.ListRepositoryConditions(cmd.Context(), repo.ProjectKey, repo.Slug)
				if err != nil {
					return err
				}
				if options.JSON {
					return writeJSON(cmd.OutOrStdout(), map[string]any{"conditions": conditions})
				}
				printReviewerConditions(cmd, conditions)
				return nil
			}

			if projectKey == "" {
				projectKey = cfg.ProjectKey
			}
			if projectKey == "" {
				return fmt.Errorf("project key is required (use --project or --repo)")
			}

			conditions, err := service.ListProjectConditions(cmd.Context(), projectKey)
			if err != nil {
				return err
			}
			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"conditions": conditions})
			}
			printReviewerConditions(cmd, conditions)
			return nil
		},
	}
	conditionCmd.AddCommand(listCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a default reviewer condition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := reviewerservice.NewService(client)
			id := args[0]

			if repositorySelector != "" {
				repo, err := resolveRepositoryReference(repositorySelector, cfg)
				if err != nil {
					return err
				}
				if err := service.DeleteRepositoryCondition(cmd.Context(), repo.ProjectKey, repo.Slug, id); err != nil {
					return err
				}
				if options.JSON {
					return writeJSON(cmd.OutOrStdout(), map[string]string{"status": "ok", "id": id})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Deleted condition %s for repository %s/%s\n", id, repo.ProjectKey, repo.Slug)
				return nil
			}

			if projectKey == "" {
				projectKey = cfg.ProjectKey
			}
			if projectKey == "" {
				return fmt.Errorf("project key is required (use --project or --repo)")
			}

			if err := service.DeleteProjectCondition(cmd.Context(), projectKey, id); err != nil {
				return err
			}
			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"status": "ok", "id": id})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted condition %s for project %s\n", id, projectKey)
			return nil
		},
	}
	conditionCmd.AddCommand(deleteCmd)

	createCmd := &cobra.Command{
		Use:   "create <json-config>",
		Short: "Create a default reviewer condition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := reviewerservice.NewService(client)
			var condition openapigenerated.RestDefaultReviewersRequest
			if err := json.Unmarshal([]byte(args[0]), &condition); err != nil {
				return fmt.Errorf("invalid condition JSON: %w", err)
			}

			if repositorySelector != "" {
				repo, err := resolveRepositoryReference(repositorySelector, cfg)
				if err != nil {
					return err
				}
				created, err := service.CreateRepositoryCondition(cmd.Context(), repo.ProjectKey, repo.Slug, condition)
				if err != nil {
					return err
				}
				return writeJSON(cmd.OutOrStdout(), created)
			}

			if projectKey == "" {
				projectKey = cfg.ProjectKey
			}
			created, err := service.CreateProjectCondition(cmd.Context(), projectKey, condition)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), created)
		},
	}
	conditionCmd.AddCommand(createCmd)

	updateCmd := &cobra.Command{
		Use:   "update <id> <json-config>",
		Short: "Update a default reviewer condition",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := reviewerservice.NewService(client)
			id := args[0]
			configJSON := []byte(args[1])

			if repositorySelector != "" {
				repo, err := resolveRepositoryReference(repositorySelector, cfg)
				if err != nil {
					return err
				}
				var condition openapigenerated.UpdatePullRequestCondition1JSONRequestBody
				if err := json.Unmarshal(configJSON, &condition); err != nil {
					return fmt.Errorf("invalid condition JSON: %w", err)
				}
				updated, err := service.UpdateRepositoryCondition(cmd.Context(), repo.ProjectKey, repo.Slug, id, condition)
				if err != nil {
					return err
				}
				return writeJSON(cmd.OutOrStdout(), updated)
			}

			if projectKey == "" {
				projectKey = cfg.ProjectKey
			}
			var condition openapigenerated.UpdatePullRequestConditionJSONRequestBody
			if err := json.Unmarshal(configJSON, &condition); err != nil {
				return fmt.Errorf("invalid condition JSON: %w", err)
			}
			updated, err := service.UpdateProjectCondition(cmd.Context(), projectKey, id, condition)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), updated)
		},
	}
	conditionCmd.AddCommand(updateCmd)

	reviewerCmd.AddCommand(conditionCmd)

	return reviewerCmd
}

func printReviewerConditions(cmd *cobra.Command, conditions []openapigenerated.RestPullRequestCondition) {
	if len(conditions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No conditions found")
		return
	}
	// Basic summary for human output
	fmt.Fprintf(cmd.OutOrStdout(), "Found %d conditions\n", len(conditions))
	// We could add more details here if we cast to RestPullRequestCondition
}
