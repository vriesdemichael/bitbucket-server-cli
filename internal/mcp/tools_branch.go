package mcp

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	branchservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/branch"
	commitservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/commit"
)

func specListBranches() Spec {
	tool := mcpgo.NewTool("list_branches",
		mcpgo.WithDescription("List branches in a repository. Use to discover existing branches before creating a new one or a pull request."),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Bitbucket project key")),
		mcpgo.WithString("repo", mcpgo.Required(), mcpgo.Description("Repository slug")),
		mcpgo.WithString("filter", mcpgo.Description("Text filter applied to branch names")),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of results (default 25)")),
	)
	return Spec{Tool: tool, Handler: func(c Clients) server.ToolHandlerFunc {
		svc := branchservice.NewService(c.OpenAPI)
		return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			project, _ := req.RequireString("project")
			repo, _ := req.RequireString("repo")
			branches, err := svc.List(ctx,
				branchservice.RepositoryRef{ProjectKey: project, Slug: repo},
				branchservice.ListOptions{
					FilterText: req.GetString("filter", ""),
					Limit:      req.GetInt("limit", 25),
				},
			)
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("list_branches failed", err), nil
			}
			return resultJSON(branches)
		}
	}}
}

func specResolveRef() Spec {
	tool := mcpgo.NewTool("resolve_ref",
		mcpgo.WithDescription("Resolve a branch or tag name to its tip commit SHA. Use as a cheap existence check before cloning or creating a pull request."),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Bitbucket project key")),
		mcpgo.WithString("repo", mcpgo.Required(), mcpgo.Description("Repository slug")),
		mcpgo.WithString("ref", mcpgo.Required(), mcpgo.Description("Branch or tag name (e.g. main, v1.2.3)")),
	)
	return Spec{Tool: tool, Handler: func(c Clients) server.ToolHandlerFunc {
		svc := commitservice.NewService(c.OpenAPI)
		return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			project, _ := req.RequireString("project")
			repo, _ := req.RequireString("repo")
			ref, _ := req.RequireString("ref")
			refs, err := svc.ListTagsAndBranches(ctx,
				commitservice.RepositoryRef{ProjectKey: project, Slug: repo},
				ref,
			)
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("resolve_ref failed", err), nil
			}
			return resultJSON(refs)
		}
	}}
}
