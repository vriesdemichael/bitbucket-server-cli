package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	reposervice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/repository"
)

func newRepoAdminCommand(options *rootOptions) *cobra.Command {
	var repositorySelector string

	repoAdminCmd := &cobra.Command{
		Use:   "admin",
		Short: "Repository administration commands (create/fork/update/delete)",
	}

	repoAdminCmd.PersistentFlags().StringVar(&repositorySelector, "repo", "", "Repository as PROJECT/slug (defaults to BITBUCKET_PROJECT_KEY + BITBUCKET_REPO_SLUG)")

	var createProject string
	var createName string
	var createDesc string
	var createForkable bool
	var createDefaultBranch string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			if strings.TrimSpace(createProject) == "" {
				return fmt.Errorf("project key is required")
			}

			service := reposervice.NewAdminService(client)
			created, err := service.Create(cmd.Context(), createProject, reposervice.CreateInput{
				Name:          createName,
				Description:   createDesc,
				Forkable:      createForkable,
				DefaultBranch: createDefaultBranch,
			})
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": created})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created repository %s/%s\n", createProject, safeString(created.Name))
			return nil
		},
	}
	createCmd.Flags().StringVar(&createProject, "project", "", "Project key")
	createCmd.Flags().StringVar(&createName, "name", "", "Repository name")
	createCmd.Flags().StringVar(&createDesc, "description", "", "Repository description")
	createCmd.Flags().BoolVar(&createForkable, "forkable", true, "Repository forkable")
	createCmd.Flags().StringVar(&createDefaultBranch, "default-branch", "", "Repository default branch")
	_ = createCmd.MarkFlagRequired("project")
	_ = createCmd.MarkFlagRequired("name")
	repoAdminCmd.AddCommand(createCmd)

	var forkName string
	var forkProject string
	forkCmd := &cobra.Command{
		Use:   "fork",
		Short: "Fork a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repoRef, err := resolveRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			repo := reposervice.RepositoryRef{ProjectKey: repoRef.ProjectKey, Slug: repoRef.Slug}
			service := reposervice.NewAdminService(client)

			forked, err := service.Fork(cmd.Context(), repo, reposervice.ForkInput{
				Name:    forkName,
				Project: forkProject,
			})
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": forked})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Forked repository to %s\n", safeString(forked.Name))
			return nil
		},
	}
	forkCmd.Flags().StringVar(&forkName, "name", "", "Name of the new fork")
	forkCmd.Flags().StringVar(&forkProject, "project", "", "Project key of the new fork")
	repoAdminCmd.AddCommand(forkCmd)

	var updateName string
	var updateDesc string
	var updateDefaultBranch string
	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update repository metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repoRef, err := resolveRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			repo := reposervice.RepositoryRef{ProjectKey: repoRef.ProjectKey, Slug: repoRef.Slug}
			service := reposervice.NewAdminService(client)

			updated, err := service.Update(cmd.Context(), repo, reposervice.UpdateInput{
				Name:          updateName,
				Description:   updateDesc,
				DefaultBranch: updateDefaultBranch,
			})
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"repository": updated})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated repository %s\n", safeString(updated.Name))
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateName, "name", "", "Repository name")
	updateCmd.Flags().StringVar(&updateDesc, "description", "", "Repository description")
	updateCmd.Flags().StringVar(&updateDefaultBranch, "default-branch", "", "Repository default branch")
	repoAdminCmd.AddCommand(updateCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			repoRef, err := resolveRepositoryReference(repositorySelector, cfg)
			if err != nil {
				return err
			}

			repo := reposervice.RepositoryRef{ProjectKey: repoRef.ProjectKey, Slug: repoRef.Slug}
			service := reposervice.NewAdminService(client)

			if err := service.Delete(cmd.Context(), repo); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"status": "ok", "repository": repoRef.ProjectKey + "/" + repoRef.Slug})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted repository %s/%s\n", repoRef.ProjectKey, repoRef.Slug)
			return nil
		},
	}
	repoAdminCmd.AddCommand(deleteCmd)

	return repoAdminCmd
}
