package cli

import (
	"fmt"
	"strings"

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

	var projectPermissionsLimit int
	permissionsCmd := &cobra.Command{Use: "permissions", Short: "Project permissions"}

	permissionsUsersCmd := &cobra.Command{Use: "users", Short: "User permissions"}
	permissionsUsersListCmd := &cobra.Command{
		Use:   "list <key>",
		Short: "List users with project permissions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := projectservice.NewService(client)
			users, err := service.ListProjectPermissionUsers(cmd.Context(), args[0], projectPermissionsLimit)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"users": users})
			}
			if len(users) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No users with project permissions found")
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
	permissionsUsersListCmd.Flags().IntVar(&projectPermissionsLimit, "limit", 100, "Page size for listing permission users")

	permissionsUsersGrantCmd := &cobra.Command{
		Use:   "grant <key> <username> <permission>",
		Short: "Grant a project permission to a user",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := projectservice.NewService(client)
			if err := service.GrantProjectUserPermission(cmd.Context(), args[0], args[1], args[2]); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "project": args[0], "username": args[1], "permission": strings.ToUpper(args[2])})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Granted %s to %s for project %s\n", strings.ToUpper(args[2]), args[1], args[0])
			return nil
		},
	}

	permissionsUsersRevokeCmd := &cobra.Command{
		Use:   "revoke <key> <username>",
		Short: "Revoke a project permission from a user",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := projectservice.NewService(client)
			if err := service.RevokeProjectUserPermission(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "project": args[0], "username": args[1]})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Revoked permissions for %s on project %s\n", args[1], args[0])
			return nil
		},
	}

	permissionsUsersCmd.AddCommand(permissionsUsersListCmd)
	permissionsUsersCmd.AddCommand(permissionsUsersGrantCmd)
	permissionsUsersCmd.AddCommand(permissionsUsersRevokeCmd)
	permissionsCmd.AddCommand(permissionsUsersCmd)

	permissionsGroupsCmd := &cobra.Command{Use: "groups", Short: "Group permissions"}
	permissionsGroupsListCmd := &cobra.Command{
		Use:   "list <key>",
		Short: "List groups with project permissions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := projectservice.NewService(client)
			groups, err := service.ListProjectPermissionGroups(cmd.Context(), args[0], projectPermissionsLimit)
			if err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"groups": groups})
			}
			if len(groups) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No groups with project permissions found")
				return nil
			}
			for _, group := range groups {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", group.Name, group.Permission)
			}

			return nil
		},
	}
	permissionsGroupsListCmd.Flags().IntVar(&projectPermissionsLimit, "limit", 100, "Page size for listing permission groups")

	permissionsGroupsGrantCmd := &cobra.Command{
		Use:   "grant <key> <group> <permission>",
		Short: "Grant a project permission to a group",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := projectservice.NewService(client)
			if err := service.GrantProjectGroupPermission(cmd.Context(), args[0], args[1], args[2]); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "project": args[0], "group": args[1], "permission": strings.ToUpper(args[2])})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Granted %s to group %s for project %s\n", strings.ToUpper(args[2]), args[1], args[0])
			return nil
		},
	}

	permissionsGroupsRevokeCmd := &cobra.Command{
		Use:   "revoke <key> <group>",
		Short: "Revoke a project permission from a group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := projectservice.NewService(client)
			if err := service.RevokeProjectGroupPermission(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"status": "ok", "project": args[0], "group": args[1]})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Revoked permissions for group %s on project %s\n", args[1], args[0])
			return nil
		},
	}

	permissionsGroupsCmd.AddCommand(permissionsGroupsListCmd)
	permissionsGroupsCmd.AddCommand(permissionsGroupsGrantCmd)
	permissionsGroupsCmd.AddCommand(permissionsGroupsRevokeCmd)
	permissionsCmd.AddCommand(permissionsGroupsCmd)

	projectCmd.AddCommand(permissionsCmd)

	return projectCmd
}
