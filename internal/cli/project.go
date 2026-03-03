package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	projectservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/project"
)

func newProjectCommand(options *rootOptions) *cobra.Command {
	var limit int

	projectCmd := &cobra.Command{
		Use:   "project",
		Short: "Project administration commands",
	}

	projectCmd.PersistentFlags().IntVar(&limit, "limit", 25, "Page size for list operations")

	var listName string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := projectservice.NewService(client)
			projects, err := service.List(cmd.Context(), projectservice.ListOptions{
				Limit: limit,
				Name:  listName,
			})
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"projects": projects})
			}

			if len(projects) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No projects found")
				return nil
			}

			for _, p := range projects {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", safeString(p.Key), safeString(p.Name))
			}

			return nil
		},
	}
	listCmd.Flags().StringVar(&listName, "name", "", "Filter projects by name")
	projectCmd.AddCommand(listCmd)

	getCmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get project details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := projectservice.NewService(client)
			project, err := service.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"project": project})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Key: %s\n", safeString(project.Key))
			fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\n", safeString(project.Name))
			fmt.Fprintf(cmd.OutOrStdout(), "Description: %s\n", safeString(project.Description))
			return nil
		},
	}
	projectCmd.AddCommand(getCmd)

	var createName string
	var createDesc string
	createCmd := &cobra.Command{
		Use:   "create <key>",
		Short: "Create a new project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := projectservice.NewService(client)
			created, err := service.Create(cmd.Context(), projectservice.CreateInput{
				Key:         args[0],
				Name:        createName,
				Description: createDesc,
			})
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"project": created})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created project %s\n", safeString(created.Key))
			return nil
		},
	}
	createCmd.Flags().StringVar(&createName, "name", "", "Project name (required)")
	createCmd.Flags().StringVar(&createDesc, "description", "", "Project description")
	_ = createCmd.MarkFlagRequired("name")
	projectCmd.AddCommand(createCmd)

	var updateName string
	var updateDesc string
	updateCmd := &cobra.Command{
		Use:   "update <key>",
		Short: "Update project details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := projectservice.NewService(client)
			updated, err := service.Update(cmd.Context(), args[0], projectservice.UpdateInput{
				Name:        updateName,
				Description: updateDesc,
			})
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"project": updated})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated project %s\n", safeString(updated.Key))
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateName, "name", "", "Project name")
	updateCmd.Flags().StringVar(&updateDesc, "description", "", "Project description")
	projectCmd.AddCommand(updateCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <key>",
		Short: "Delete a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := projectservice.NewService(client)
			if err := service.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"status": "ok", "project_key": args[0]})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted project %s\n", args[0])
			return nil
		},
	}
	projectCmd.AddCommand(deleteCmd)

	return projectCmd
}
