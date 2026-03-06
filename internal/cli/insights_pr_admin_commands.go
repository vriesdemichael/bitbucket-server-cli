package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
	pullrequestservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/pullrequest"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/transport/httpclient"
)

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
			repo, service, err := loadQualityRepoAndService(repositorySelector)
			if err != nil {
				return err
			}

			request := openapigenerated.SetACodeInsightsReportJSONRequestBody{}
			if err := json.Unmarshal([]byte(reportBody), &request); err != nil {
				return apperrors.New(apperrors.KindValidation, "invalid JSON for --body", err)
			}

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
			repo, service, err := loadQualityRepoAndService(repositorySelector)
			if err != nil {
				return err
			}

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
			repo, service, err := loadQualityRepoAndService(repositorySelector)
			if err != nil {
				return err
			}

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
			repo, service, err := loadQualityRepoAndService(repositorySelector)
			if err != nil {
				return err
			}

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
			repo, service, err := loadQualityRepoAndService(repositorySelector)
			if err != nil {
				return err
			}

			annotations := make([]openapigenerated.RestSingleAddInsightAnnotationRequest, 0)
			if err := json.Unmarshal([]byte(annotationBody), &annotations); err != nil {
				return apperrors.New(apperrors.KindValidation, "invalid JSON for --body (expected array of annotations)", err)
			}

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
			repo, service, err := loadQualityRepoAndService(repositorySelector)
			if err != nil {
				return err
			}

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
			repo, service, err := loadQualityRepoAndService(repositorySelector)
			if err != nil {
				return err
			}

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

func newPRCommand(options *rootOptions) *cobra.Command {
	var repository string
	var state string
	var limit int
	var start int
	var sourceBranch string
	var targetBranch string

	prCmd := &cobra.Command{
		Use:   "pr",
		Short: "Pull request commands",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List pull requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))
			pullRequests, err := service.List(cmd.Context(), repo, pullrequestservice.ListOptions{
				State:        state,
				Limit:        limit,
				Start:        start,
				SourceBranch: sourceBranch,
				TargetBranch: targetBranch,
			})
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"repository":    repo,
					"filters":       map[string]any{"state": strings.ToLower(strings.TrimSpace(state)), "start": start, "limit": limit, "source_branch": sourceBranch, "target_branch": targetBranch},
					"pull_requests": pullRequests,
				})
			}

			if len(pullRequests) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No pull requests found")
				return nil
			}

			for _, pullRequest := range pullRequests {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"#%d\t%s\t%s -> %s\t%s\n",
					pullRequest.ID,
					pullRequest.State,
					pullRequest.SourceBranch,
					pullRequest.TargetBranch,
					pullRequest.Title,
				)
			}

			return nil
		},
	}

	listCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	listCmd.Flags().StringVar(&state, "state", "open", "Pull request state filter: open, closed, all")
	listCmd.Flags().IntVar(&limit, "limit", 25, "Page size for Bitbucket pull request list operations")
	listCmd.Flags().IntVar(&start, "start", 0, "Start offset for Bitbucket pull request list operations")
	listCmd.Flags().StringVar(&sourceBranch, "source-branch", "", "Optional source branch filter")
	listCmd.Flags().StringVar(&targetBranch, "target-branch", "", "Optional target branch filter")
	prCmd.AddCommand(listCmd)

	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get pull request details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))
			pullRequest, err := service.Get(cmd.Context(), repo, args[0])
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": repo, "pull_request": pullRequest})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "#%d\t%s\t%s -> %s\t%s\n", pullRequest.ID, pullRequest.State, pullRequest.SourceBranch, pullRequest.TargetBranch, pullRequest.Title)
			if len(pullRequest.Reviewers) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Reviewers: %d\n", len(pullRequest.Reviewers))
			}

			return nil
		},
	}
	getCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	prCmd.AddCommand(getCmd)

	var createFromRef string
	var createToRef string
	var createTitle string
	var createDescription string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pull request",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))
			created, err := service.Create(cmd.Context(), repo, pullrequestservice.CreateInput{
				FromRef:     createFromRef,
				ToRef:       createToRef,
				Title:       createTitle,
				Description: createDescription,
			})
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": repo, "pull_request": created})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created pull request #%d\n", created.ID)
			return nil
		},
	}
	createCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	createCmd.Flags().StringVar(&createFromRef, "from-ref", "", "Source branch (name or refs/heads/name)")
	createCmd.Flags().StringVar(&createToRef, "to-ref", "", "Target branch (name or refs/heads/name)")
	createCmd.Flags().StringVar(&createTitle, "title", "", "Pull request title")
	createCmd.Flags().StringVar(&createDescription, "description", "", "Pull request description")
	_ = createCmd.MarkFlagRequired("from-ref")
	_ = createCmd.MarkFlagRequired("to-ref")
	_ = createCmd.MarkFlagRequired("title")
	prCmd.AddCommand(createCmd)

	var updateTitle string
	var updateDescription string
	var updateVersion int
	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update pull request metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))
			updated, err := service.Update(cmd.Context(), repo, args[0], pullrequestservice.UpdateInput{
				Title:       updateTitle,
				Description: updateDescription,
				Version:     updateVersion,
			})
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": repo, "pull_request": updated})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated pull request #%d\n", updated.ID)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	updateCmd.Flags().StringVar(&updateTitle, "title", "", "Updated pull request title")
	updateCmd.Flags().StringVar(&updateDescription, "description", "", "Updated pull request description")
	updateCmd.Flags().IntVar(&updateVersion, "version", 0, "Expected pull request version")
	_ = updateCmd.MarkFlagRequired("version")
	prCmd.AddCommand(updateCmd)

	var transitionVersion int
	mergeCmd := &cobra.Command{
		Use:   "merge <id>",
		Short: "Merge a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))

			var version *int
			if cmd.Flags().Changed("version") {
				version = &transitionVersion
			}

			merged, err := service.Merge(cmd.Context(), repo, args[0], version)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": repo, "pull_request": merged})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Merged pull request #%d\n", merged.ID)
			return nil
		},
	}
	mergeCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	mergeCmd.Flags().IntVar(&transitionVersion, "version", 0, "Expected pull request version")
	prCmd.AddCommand(mergeCmd)

	declineCmd := &cobra.Command{
		Use:   "decline <id>",
		Short: "Decline a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))

			var version *int
			if cmd.Flags().Changed("version") {
				version = &transitionVersion
			}

			declined, err := service.Decline(cmd.Context(), repo, args[0], version)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": repo, "pull_request": declined})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Declined pull request #%d\n", declined.ID)
			return nil
		},
	}
	declineCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	declineCmd.Flags().IntVar(&transitionVersion, "version", 0, "Expected pull request version")
	prCmd.AddCommand(declineCmd)

	reopenCmd := &cobra.Command{
		Use:   "reopen <id>",
		Short: "Reopen a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))

			var version *int
			if cmd.Flags().Changed("version") {
				version = &transitionVersion
			}

			reopened, err := service.Reopen(cmd.Context(), repo, args[0], version)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": repo, "pull_request": reopened})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Reopened pull request #%d\n", reopened.ID)
			return nil
		},
	}
	reopenCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	reopenCmd.Flags().IntVar(&transitionVersion, "version", 0, "Expected pull request version")
	prCmd.AddCommand(reopenCmd)

	reviewCmd := &cobra.Command{Use: "review", Short: "Pull request review commands"}

	reviewApproveCmd := &cobra.Command{
		Use:   "approve <id>",
		Short: "Approve a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))
			pullRequest, err := service.Approve(cmd.Context(), repo, args[0])
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": repo, "pull_request": pullRequest})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Approved pull request #%d\n", pullRequest.ID)
			return nil
		},
	}
	reviewApproveCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	reviewCmd.AddCommand(reviewApproveCmd)

	reviewUnapproveCmd := &cobra.Command{
		Use:   "unapprove <id>",
		Short: "Remove pull request approval",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))
			pullRequest, err := service.Unapprove(cmd.Context(), repo, args[0])
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": repo, "pull_request": pullRequest})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed approval for pull request #%d\n", pullRequest.ID)
			return nil
		},
	}
	reviewUnapproveCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	reviewCmd.AddCommand(reviewUnapproveCmd)

	reviewerCmd := &cobra.Command{Use: "reviewer", Short: "Manage pull request reviewers"}
	var reviewerUsername string
	reviewerAddCmd := &cobra.Command{
		Use:   "add <id>",
		Short: "Add a reviewer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))
			pullRequest, err := service.AddReviewer(cmd.Context(), repo, args[0], reviewerUsername)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": repo, "pull_request": pullRequest})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added reviewer %s to pull request #%d\n", reviewerUsername, pullRequest.ID)
			return nil
		},
	}
	reviewerAddCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	reviewerAddCmd.Flags().StringVar(&reviewerUsername, "user", "", "Reviewer username")
	_ = reviewerAddCmd.MarkFlagRequired("user")
	reviewerCmd.AddCommand(reviewerAddCmd)

	reviewerRemoveCmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a reviewer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))
			pullRequest, err := service.RemoveReviewer(cmd.Context(), repo, args[0], reviewerUsername)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": repo, "pull_request": pullRequest})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed reviewer %s from pull request #%d\n", reviewerUsername, pullRequest.ID)
			return nil
		},
	}
	reviewerRemoveCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	reviewerRemoveCmd.Flags().StringVar(&reviewerUsername, "user", "", "Reviewer username")
	_ = reviewerRemoveCmd.MarkFlagRequired("user")
	reviewerCmd.AddCommand(reviewerRemoveCmd)

	reviewCmd.AddCommand(reviewerCmd)
	prCmd.AddCommand(reviewCmd)

	taskCmd := &cobra.Command{Use: "task", Short: "Pull request task commands"}

	var taskState string
	var taskLimit int
	var taskStart int
	taskListCmd := &cobra.Command{
		Use:   "list <id>",
		Short: "List tasks for a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))
			tasks, err := service.ListTasks(cmd.Context(), repo, args[0], pullrequestservice.TaskListOptions{State: taskState, Limit: taskLimit, Start: taskStart})
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": repo, "pull_request_id": args[0], "tasks": tasks})
			}

			if len(tasks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No tasks found")
				return nil
			}

			for _, task := range tasks {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\n", task.ID, task.State, task.Text)
			}

			return nil
		},
	}
	taskListCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	taskListCmd.Flags().StringVar(&taskState, "state", "open", "Task state filter: open, resolved, all")
	taskListCmd.Flags().IntVar(&taskLimit, "limit", 25, "Page size for task list operations")
	taskListCmd.Flags().IntVar(&taskStart, "start", 0, "Start offset for task list operations")
	taskCmd.AddCommand(taskListCmd)

	var taskText string
	taskCreateCmd := &cobra.Command{
		Use:   "create <id>",
		Short: "Create a task on a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))
			task, err := service.CreateTask(cmd.Context(), repo, args[0], taskText)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": repo, "pull_request_id": args[0], "task": task})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created task %d\n", task.ID)
			return nil
		},
	}
	taskCreateCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	taskCreateCmd.Flags().StringVar(&taskText, "text", "", "Task text")
	_ = taskCreateCmd.MarkFlagRequired("text")
	taskCmd.AddCommand(taskCreateCmd)

	var taskID string
	var taskResolved bool
	var taskVersion int
	taskUpdateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a task on a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			var resolved *bool
			if cmd.Flags().Changed("resolved") {
				resolved = &taskResolved
			}
			var version *int
			if cmd.Flags().Changed("version") {
				version = &taskVersion
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))
			updated, err := service.UpdateTask(cmd.Context(), repo, args[0], taskID, taskText, resolved, version)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": repo, "pull_request_id": args[0], "task": updated})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated task %d\n", updated.ID)
			return nil
		},
	}
	taskUpdateCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	taskUpdateCmd.Flags().StringVar(&taskID, "task", "", "Task ID")
	taskUpdateCmd.Flags().StringVar(&taskText, "text", "", "Task text")
	taskUpdateCmd.Flags().BoolVar(&taskResolved, "resolved", false, "Mark task as resolved/unresolved")
	taskUpdateCmd.Flags().IntVar(&taskVersion, "version", 0, "Expected task version")
	_ = taskUpdateCmd.MarkFlagRequired("task")
	taskCmd.AddCommand(taskUpdateCmd)

	taskDeleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a task from a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			repo, err := resolvePullRequestRepositoryReference(repository, cfg)
			if err != nil {
				return err
			}

			var version *int
			if cmd.Flags().Changed("version") {
				version = &taskVersion
			}

			service := pullrequestservice.NewService(httpclient.NewFromConfig(cfg))
			if err := service.DeleteTask(cmd.Context(), repo, args[0], taskID, version); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "repository": repo, "pull_request_id": args[0], "task_id": taskID})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted task %s\n", taskID)
			return nil
		},
	}
	taskDeleteCmd.Flags().StringVar(&repository, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")
	taskDeleteCmd.Flags().StringVar(&taskID, "task", "", "Task ID")
	taskDeleteCmd.Flags().IntVar(&taskVersion, "version", 0, "Expected task version")
	_ = taskDeleteCmd.MarkFlagRequired("task")
	taskCmd.AddCommand(taskDeleteCmd)

	prCmd.AddCommand(taskCmd)

	return prCmd
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
			cfg, err := loadConfig()
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
