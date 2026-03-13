package cli

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/git"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/transport/httpclient"
)

func newRepoCloneCommand(options *rootOptions) *cobra.Command {
	var noUpstream bool
	var upstreamRemoteName string

	cmd := &cobra.Command{
		Use:   "clone <repository> [directory] [-- <gitflags>...]",
		Short: "Clone a repository to the local filesystem",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			repo, cloneHost, usedURLInput, err := resolveRepositoryCloneInput(args[0], cfg)
			if err != nil {
				return err
			}

			directory, extraInput := splitCloneDirectoryAndExtraArgs(repo.Slug, args[1:])
			if directory == "" {
				return apperrors.New(apperrors.KindValidation, "clone directory cannot be empty", nil)
			}

			extraCloneArgs, err := normalizeCloneExtraArgs(extraInput)
			if err != nil {
				return err
			}

			resolvedUpstreamName, err := normalizeUpstreamRemoteName(upstreamRemoteName, repo.ProjectKey)
			if err != nil {
				return err
			}

			cloneURL, err := buildCloneURL(args[0], usedURLInput, cloneHost, repo)
			if err != nil {
				return err
			}

			backend := gitBackendFactory()
			if backend == nil {
				return apperrors.New(apperrors.KindInternal, "git backend is not configured", nil)
			}

			err = backend.Clone(cmd.Context(), cloneURL, git.CloneOptions{
				Directory: directory,
				ExtraArgs: extraCloneArgs,
			})
			if err != nil {
				return err
			}

			upstreamURL := ""
			upstreamOwner := ""
			upstreamAdded := false
			if !noUpstream {
				upstreamOwner, upstreamURL, err = lookupParentCloneURL(cmd.Context(), cfg, cloneHost, repo)
				if strings.TrimSpace(upstreamURL) != "" {
					if strings.EqualFold(strings.TrimSpace(upstreamRemoteName), "@owner") && strings.TrimSpace(upstreamOwner) != "" {
						resolvedUpstreamName = strings.ToLower(strings.TrimSpace(upstreamOwner))
					}
					if err := backend.AddRemote(cmd.Context(), directory, git.Remote{Name: resolvedUpstreamName, URL: upstreamURL}); err != nil {
						return err
					}
					upstreamAdded = true
				}
			}

			if options.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"status": "ok",
					"repository": map[string]string{
						"project_key": repo.ProjectKey,
						"slug":        repo.Slug,
					},
					"clone_url":   cloneURL,
					"directory":   directory,
					"no_upstream": noUpstream,
					"upstream": map[string]any{
						"configured": upstreamAdded,
						"name":       resolvedUpstreamName,
						"url":        upstreamURL,
					},
					"repository_input": map[string]any{
						"raw":      args[0],
						"used_url": usedURLInput,
					},
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cloned %s/%s into %s\n", repo.ProjectKey, repo.Slug, directory)
			if upstreamAdded {
				fmt.Fprintf(cmd.OutOrStdout(), "Added remote %q -> %s\n", resolvedUpstreamName, upstreamURL)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&noUpstream, "no-upstream", false, "Do not add an upstream remote when cloning a fork")
	cmd.Flags().StringVarP(&upstreamRemoteName, "upstream-remote-name", "u", "upstream", "Upstream remote name when cloning a fork")
	cmd.Flags().SetInterspersed(false)

	return cmd
}

func resolveRepositoryCloneInput(input string, cfg config.AppConfig) (repositorySelector, string, bool, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return repositorySelector{}, "", false, apperrors.New(apperrors.KindValidation, "repository is required", nil)
	}

	host, repoFromHostSelector, ok := parseHostQualifiedRepositorySelector(trimmed)
	if ok {
		hostURL := host
		if !strings.Contains(hostURL, "://") {
			hostURL = "https://" + hostURL
		}
		return repoFromHostSelector, hostURL, true, nil
	}

	host, projectKey, slug, parsed := parseBitbucketRemote(trimmed)
	if parsed {
		return repositorySelector{ProjectKey: projectKey, Slug: slug}, normalizeCloneHost(trimmed, host), true, nil
	}

	repo, parsedSelector, err := parseCloneSelector(trimmed, cfg.ProjectKey)
	if err != nil {
		return repositorySelector{}, "", false, err
	}

	if strings.TrimSpace(cfg.BitbucketURL) == "" {
		if parsedSelector == cloneSelectorProjectSlug {
			return repositorySelector{}, "", false, apperrors.New(apperrors.KindValidation, "BITBUCKET_URL is required to clone repositories by PROJECT/slug", nil)
		}
		return repositorySelector{}, "", false, apperrors.New(apperrors.KindValidation, "BITBUCKET_URL is required to clone repositories by slug", nil)
	}

	return repo, strings.TrimSpace(cfg.BitbucketURL), false, nil
}

type cloneSelectorKind int

const (
	cloneSelectorProjectSlug cloneSelectorKind = iota
	cloneSelectorSlugOnly
)

func parseCloneSelector(value, defaultProjectKey string) (repositorySelector, cloneSelectorKind, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return repositorySelector{}, cloneSelectorProjectSlug, apperrors.New(apperrors.KindValidation, "repository must be in PROJECT/slug format or a slug with BITBUCKET_PROJECT_KEY set", nil)
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) == 1 {
		project := strings.TrimSpace(defaultProjectKey)
		if project == "" {
			return repositorySelector{}, cloneSelectorSlugOnly, apperrors.New(apperrors.KindValidation, "repository must be in PROJECT/slug format when BITBUCKET_PROJECT_KEY is not set", nil)
		}
		return repositorySelector{ProjectKey: project, Slug: trimmed}, cloneSelectorSlugOnly, nil
	}

	if len(parts) != 2 {
		return repositorySelector{}, cloneSelectorProjectSlug, apperrors.New(apperrors.KindValidation, "repository must be in PROJECT/slug format", nil)
	}

	projectKey := strings.TrimSpace(parts[0])
	slug := strings.TrimSpace(parts[1])
	if projectKey == "" || slug == "" {
		return repositorySelector{}, cloneSelectorProjectSlug, apperrors.New(apperrors.KindValidation, "repository must be in PROJECT/slug format", nil)
	}

	return repositorySelector{ProjectKey: projectKey, Slug: slug}, cloneSelectorProjectSlug, nil
}

func buildCloneURL(rawInput string, usedURLInput bool, cloneHost string, repo repositorySelector) (string, error) {
	if usedURLInput && strings.Contains(strings.TrimSpace(rawInput), "://") {
		return strings.TrimSpace(rawInput), nil
	}
	return buildBitbucketCloneURL(cloneHost, repo.ProjectKey, repo.Slug)
}

func splitCloneDirectoryAndExtraArgs(defaultDirectory string, values []string) (string, []string) {
	trimmedDefault := strings.TrimSpace(defaultDirectory)
	if len(values) == 0 {
		return trimmedDefault, nil
	}

	first := strings.TrimSpace(values[0])
	if first == "" {
		return "", values[1:]
	}
	if first == "--" || strings.HasPrefix(first, "-") {
		return trimmedDefault, values
	}

	return first, values[1:]
}

func normalizeCloneHost(rawInput, parsedHost string) string {
	trimmed := strings.TrimSpace(rawInput)
	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err == nil && strings.TrimSpace(parsed.Scheme) != "" && strings.TrimSpace(parsed.Host) != "" {
			parsed.Path = ""
			parsed.RawQuery = ""
			parsed.Fragment = ""
			return strings.TrimSuffix(parsed.String(), "/")
		}
	}

	host := strings.TrimSpace(parsedHost)
	if host == "" {
		return ""
	}

	return "https://" + host
}

func normalizeCloneExtraArgs(extra []string) ([]string, error) {
	if len(extra) == 0 {
		return nil, nil
	}

	result := make([]string, 0, len(extra))
	for _, value := range extra {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || trimmed == "--" {
			continue
		}
		if !strings.HasPrefix(trimmed, "-") {
			return nil, apperrors.New(apperrors.KindValidation, "additional git clone arguments must be passed after --", nil)
		}
		result = append(result, trimmed)
	}

	return result, nil
}

func normalizeUpstreamRemoteName(name string, fallbackOwner string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", apperrors.New(apperrors.KindValidation, "--upstream-remote-name cannot be empty", nil)
	}
	if trimmed == "@owner" {
		if strings.TrimSpace(fallbackOwner) == "" {
			return "owner", nil
		}
		return strings.ToLower(strings.TrimSpace(fallbackOwner)), nil
	}
	if strings.Contains(trimmed, " ") {
		return "", apperrors.New(apperrors.KindValidation, "--upstream-remote-name cannot contain spaces", nil)
	}
	return trimmed, nil
}

func lookupParentCloneURL(ctx context.Context, cfg config.AppConfig, cloneHost string, repo repositorySelector) (string, string, error) {
	probeCfg := cfg
	probeCfg.BitbucketURL = cloneHost

	client := httpclient.NewFromConfig(probeCfg)
	path := fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s", url.PathEscape(repo.ProjectKey), url.PathEscape(repo.Slug))

	var response struct {
		Origin *struct {
			Project *struct {
				Key string `json:"key"`
			} `json:"project"`
			Slug string `json:"slug"`
		} `json:"origin"`
	}

	err := client.GetJSON(ctx, path, nil, &response)
	if err != nil {
		return "", "", nil
	}

	if response.Origin == nil || response.Origin.Project == nil {
		return "", "", nil
	}
	if strings.TrimSpace(response.Origin.Project.Key) == "" || strings.TrimSpace(response.Origin.Slug) == "" {
		return "", "", nil
	}

	parentURL, err := buildBitbucketCloneURL(cloneHost, response.Origin.Project.Key, response.Origin.Slug)
	if err != nil {
		return "", "", err
	}

	return response.Origin.Project.Key, parentURL, nil
}

func buildBitbucketCloneURL(baseURL, projectKey, slug string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
		return "", apperrors.New(apperrors.KindValidation, "BITBUCKET_URL must include a valid scheme and host", err)
	}

	trimmedProject := strings.TrimSpace(projectKey)
	trimmedSlug := strings.TrimSpace(slug)
	if trimmedProject == "" || trimmedSlug == "" {
		return "", apperrors.New(apperrors.KindValidation, "repository selector must include project key and slug", nil)
	}

	basePath := strings.TrimSuffix(parsed.Path, "/")
	parsed.Path = fmt.Sprintf("%s/scm/%s/%s.git", basePath, url.PathEscape(trimmedProject), url.PathEscape(trimmedSlug))
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed.String(), nil
}
