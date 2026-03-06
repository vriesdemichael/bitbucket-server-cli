package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	diffservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/diff"
	tagservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/tag"
)

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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveTagRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveTagRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveTagRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
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
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repo, err := resolveTagRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
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
