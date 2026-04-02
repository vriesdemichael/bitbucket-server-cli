package mcp

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	qualityservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/quality"
)

func specGetBuildStatus() Spec {
	tool := mcpgo.NewTool("get_build_status",
		mcpgo.WithDescription("Get build/CI statuses for a specific commit. Use this to check whether CI passed before declaring a PR ready to merge."),
		mcpgo.WithString("commit_id", mcpgo.Required(), mcpgo.Description("Full commit SHA")),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of results (default 25)")),
	)
	return Spec{Tool: tool, Handler: func(c Clients) server.ToolHandlerFunc {
		svc := qualityservice.NewService(c.OpenAPI)
		return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			commitID, _ := req.RequireString("commit_id")
			limit := req.GetInt("limit", 25)
			statuses, err := svc.GetBuildStatuses(ctx, commitID, limit, "")
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("get_build_status failed", err), nil
			}
			return resultJSON(statuses)
		}
	}}
}

func specSetBuildStatus() Spec {
	tool := mcpgo.NewTool("set_build_status",
		mcpgo.WithDescription("Report a build/CI status for a commit back to Bitbucket. Use this when running CI pipelines that should surface results in PR views."),
		mcpgo.WithString("commit_id", mcpgo.Required(), mcpgo.Description("Full commit SHA")),
		mcpgo.WithString("key", mcpgo.Required(), mcpgo.Description("Unique build key (e.g. my-pipeline/unit-tests)")),
		mcpgo.WithString("state", mcpgo.Required(), mcpgo.Description("Build state: SUCCESSFUL, FAILED, or INPROGRESS"),
			mcpgo.Enum("SUCCESSFUL", "FAILED", "INPROGRESS")),
		mcpgo.WithString("url", mcpgo.Required(), mcpgo.Description("URL to the build details")),
		mcpgo.WithString("name", mcpgo.Description("Human-readable build name")),
		mcpgo.WithString("description", mcpgo.Description("Build description or summary")),
	)
	return Spec{Tool: tool, Handler: func(c Clients) server.ToolHandlerFunc {
		svc := qualityservice.NewService(c.OpenAPI)
		return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			commitID, _ := req.RequireString("commit_id")
			key, _ := req.RequireString("key")
			state, _ := req.RequireString("state")
			buildURL, _ := req.RequireString("url")
			err := svc.SetBuildStatus(ctx, commitID, qualityservice.BuildStatusSetInput{
				Key:         key,
				State:       state,
				URL:         buildURL,
				Name:        req.GetString("name", ""),
				Description: req.GetString("description", ""),
			})
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("set_build_status failed", err), nil
			}
			return mcpgo.NewToolResultText("build status set"), nil
		}
	}}
}

func specListRequiredBuilds() Spec {
	tool := mcpgo.NewTool("list_required_builds",
		mcpgo.WithDescription("List required build checks that must pass before a pull request can be merged. Check this before attempting a merge to understand what CI must succeed."),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Bitbucket project key")),
		mcpgo.WithString("repo", mcpgo.Required(), mcpgo.Description("Repository slug")),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of results (default 25)")),
	)
	return Spec{Tool: tool, Handler: func(c Clients) server.ToolHandlerFunc {
		svc := qualityservice.NewService(c.OpenAPI)
		return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			project, _ := req.RequireString("project")
			repo, _ := req.RequireString("repo")
			limit := req.GetInt("limit", 25)
			checks, err := svc.ListRequiredBuildChecks(ctx,
				qualityservice.RepositoryRef{ProjectKey: project, Slug: repo},
				limit,
			)
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("list_required_builds failed", err), nil
			}
			return resultJSON(checks)
		}
	}}
}
