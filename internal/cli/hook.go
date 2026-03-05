package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
	hookservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/hook"
)

func newHookCommand(options *rootOptions) *cobra.Command {
	var projectKey string
	var repositorySelector string

	hookCmd := &cobra.Command{
		Use:   "hook",
		Short: "Manage repository/project hooks",
	}

	hookCmd.PersistentFlags().StringVar(&projectKey, "project", "", "Project key")
	hookCmd.PersistentFlags().StringVar(&repositorySelector, "repo", "", "Repository as PROJECT/slug")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := hookservice.NewService(client)

			if repositorySelector != "" {
				repo, err := resolveRepositoryReference(repositorySelector, cfg)
				if err != nil {
					return err
				}
				hooks, err := service.ListRepositoryHooks(cmd.Context(), repo.ProjectKey, repo.Slug, 100)
				if err != nil {
					return err
				}
				if options.JSON {
					return writeJSON(cmd.OutOrStdout(), map[string]any{"hooks": hooks})
				}
				printHooks(cmd, hooks)
				return nil
			}

			if projectKey == "" {
				projectKey = cfg.ProjectKey
			}
			if projectKey == "" {
				return fmt.Errorf("project key is required (use --project or --repo)")
			}

			hooks, err := service.ListProjectHooks(cmd.Context(), projectKey, 100)
			if err != nil {
				return err
			}
			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"hooks": hooks})
			}
			printHooks(cmd, hooks)
			return nil
		},
	}
	hookCmd.AddCommand(listCmd)

	enableCmd := &cobra.Command{
		Use:   "enable <hook-key>",
		Short: "Enable a hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := hookservice.NewService(client)
			hookKey := args[0]

			if repositorySelector != "" {
				repo, err := resolveRepositoryReference(repositorySelector, cfg)
				if err != nil {
					return err
				}
				hook, err := service.EnableRepositoryHook(cmd.Context(), repo.ProjectKey, repo.Slug, hookKey)
				if err != nil {
					return err
				}
				if options.JSON {
					return writeJSON(cmd.OutOrStdout(), map[string]any{"hook": hook})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Enabled hook %s for repository %s/%s\n", hookKey, repo.ProjectKey, repo.Slug)
				return nil
			}

			if projectKey == "" {
				projectKey = cfg.ProjectKey
			}
			if projectKey == "" {
				return fmt.Errorf("project key is required (use --project or --repo)")
			}

			hook, err := service.EnableProjectHook(cmd.Context(), projectKey, hookKey)
			if err != nil {
				return err
			}
			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"hook": hook})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Enabled hook %s for project %s\n", hookKey, projectKey)
			return nil
		},
	}
	hookCmd.AddCommand(enableCmd)

	disableCmd := &cobra.Command{
		Use:   "disable <hook-key>",
		Short: "Disable a hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := hookservice.NewService(client)
			hookKey := args[0]

			if repositorySelector != "" {
				repo, err := resolveRepositoryReference(repositorySelector, cfg)
				if err != nil {
					return err
				}
				if err := service.DisableRepositoryHook(cmd.Context(), repo.ProjectKey, repo.Slug, hookKey); err != nil {
					return err
				}
				if options.JSON {
					return writeJSON(cmd.OutOrStdout(), map[string]string{"status": "ok"})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Disabled hook %s for repository %s/%s\n", hookKey, repo.ProjectKey, repo.Slug)
				return nil
			}

			if projectKey == "" {
				projectKey = cfg.ProjectKey
			}
			if projectKey == "" {
				return fmt.Errorf("project key is required (use --project or --repo)")
			}

			if err := service.DisableProjectHook(cmd.Context(), projectKey, hookKey); err != nil {
				return err
			}
			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"status": "ok"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Disabled hook %s for project %s\n", hookKey, projectKey)
			return nil
		},
	}
	hookCmd.AddCommand(disableCmd)

	var configFile string
	configureCmd := &cobra.Command{
		Use:   "configure <hook-key> [settings-json]",
		Short: "Configure hook settings",
		Long:  "Configure hook settings using JSON from argument, file (--config-file), or stdin (-)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := loadConfigAndClient()
			if err != nil {
				return err
			}

			service := hookservice.NewService(client)
			if len(args) < 1 {
				return fmt.Errorf("hook key is required")
			}
			hookKey := args[0]

			var settingsData []byte
			hasArgSettings := len(args) > 1
			hasConfigFile := configFile != ""

			if hasArgSettings && hasConfigFile {
				return fmt.Errorf("cannot provide settings as both an argument and via --config-file; please use only one")
			}

			if hasArgSettings {
				if args[1] == "-" {
					settingsData, err = io.ReadAll(cmd.InOrStdin())
					if err != nil {
						return fmt.Errorf("failed to read stdin: %w", err)
					}
				} else {
					settingsData = []byte(args[1])
				}
			} else if hasConfigFile {
				settingsData, err = os.ReadFile(configFile)
				if err != nil {
					return fmt.Errorf("failed to read config file: %w", err)
				}
			} else {
				// No settings provided, just get current settings
				if repositorySelector != "" {
					repo, err := resolveRepositoryReference(repositorySelector, cfg)
					if err != nil {
						return err
					}
					settings, err := service.GetRepositoryHookSettings(cmd.Context(), repo.ProjectKey, repo.Slug, hookKey)
					if err != nil {
						return err
					}
					return writeJSON(cmd.OutOrStdout(), settings)
				}
				if projectKey == "" {
					projectKey = cfg.ProjectKey
				}
				if projectKey == "" {
					return fmt.Errorf("project key is required (use --project or --repo)")
				}
				settings, err := service.GetProjectHookSettings(cmd.Context(), projectKey, hookKey)
				if err != nil {
					return err
				}
				return writeJSON(cmd.OutOrStdout(), settings)
			}

			var settings map[string]any
			if err := json.Unmarshal(settingsData, &settings); err != nil {
				return fmt.Errorf("invalid settings JSON: %w", err)
			}
			if settings == nil {
				return fmt.Errorf("invalid settings JSON: settings must be a JSON object, not null")
			}

			if repositorySelector != "" {
				repo, err := resolveRepositoryReference(repositorySelector, cfg)
				if err != nil {
					return err
				}
				result, err := service.SetRepositoryHookSettings(cmd.Context(), repo.ProjectKey, repo.Slug, hookKey, settings)
				if err != nil {
					return err
				}
				if options.JSON {
					return writeJSON(cmd.OutOrStdout(), result)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Configured hook %s for repository %s/%s\n", hookKey, repo.ProjectKey, repo.Slug)
				return nil
			}

			if projectKey == "" {
				projectKey = cfg.ProjectKey
			}
			if projectKey == "" {
				return fmt.Errorf("project key is required (use --project or --repo)")
			}
			result, err := service.SetProjectHookSettings(cmd.Context(), projectKey, hookKey, settings)
			if err != nil {
				return err
			}
			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Configured hook %s for project %s\n", hookKey, projectKey)
			return nil
		},
	}
	configureCmd.Flags().StringVar(&configFile, "config-file", "", "JSON file containing hook settings")
	hookCmd.AddCommand(configureCmd)

	return hookCmd
}

func printHooks(cmd *cobra.Command, hooks []openapigenerated.RestRepositoryHook) {
	if len(hooks) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No hooks found")
		return
	}
	for _, h := range hooks {
		enabled := "disabled"
		if h.Enabled != nil && *h.Enabled {
			enabled = "enabled"
		}
		name := ""
		if h.Details != nil && h.Details.Name != nil {
			name = *h.Details.Name
		}
		key := ""
		if h.Details != nil && h.Details.Key != nil {
			key = *h.Details.Key
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", key, enabled, name)
	}
}
