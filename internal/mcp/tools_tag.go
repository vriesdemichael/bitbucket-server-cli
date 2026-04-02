package mcp

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	tagservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/tag"
)

func specListTags() Spec {
	tool := mcpgo.NewTool("list_tags",
		mcpgo.WithDescription("List tags in a repository. Use to find the latest release baseline or versioning information."),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Bitbucket project key")),
		mcpgo.WithString("repo", mcpgo.Required(), mcpgo.Description("Repository slug")),
		mcpgo.WithString("filter", mcpgo.Description("Text filter applied to tag names")),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of results (default 25)")),
	)
	return Spec{Tool: tool, Handler: func(c Clients) server.ToolHandlerFunc {
		svc := tagservice.NewService(c.OpenAPI)
		return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			project, _ := req.RequireString("project")
			repo, _ := req.RequireString("repo")
			tags, err := svc.List(ctx,
				tagservice.RepositoryRef{ProjectKey: project, Slug: repo},
				tagservice.ListOptions{
					FilterText: req.GetString("filter", ""),
					Limit:      req.GetInt("limit", 25),
				},
			)
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("list_tags failed", err), nil
			}
			return resultJSON(tags)
		}
	}}
}

func specCreateTag() Spec {
	tool := mcpgo.NewTool("create_tag",
		mcpgo.WithDescription("Create a tag on a specific commit or ref. Use for release tagging after a PR is merged."),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Bitbucket project key")),
		mcpgo.WithString("repo", mcpgo.Required(), mcpgo.Description("Repository slug")),
		mcpgo.WithString("name", mcpgo.Required(), mcpgo.Description("Tag name (e.g. v1.2.3)")),
		mcpgo.WithString("start_point", mcpgo.Required(), mcpgo.Description("Branch name or commit SHA to tag")),
		mcpgo.WithString("message", mcpgo.Description("Optional annotated tag message; omit for a lightweight tag")),
	)
	return Spec{Tool: tool, Handler: func(c Clients) server.ToolHandlerFunc {
		svc := tagservice.NewService(c.OpenAPI)
		return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			project, _ := req.RequireString("project")
			repo, _ := req.RequireString("repo")
			name, _ := req.RequireString("name")
			startPoint, _ := req.RequireString("start_point")
			tag, err := svc.Create(ctx,
				tagservice.RepositoryRef{ProjectKey: project, Slug: repo},
				name,
				startPoint,
				req.GetString("message", ""),
			)
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("create_tag failed", err), nil
			}
			return resultJSON(tag)
		}
	}}
}
