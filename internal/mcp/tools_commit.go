package mcp

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	commitservice "github.com/vriesdemichael/bitbucket-server-cli/internal/services/commit"
)

func specListCommits() Spec {
	tool := mcpgo.NewTool("list_commits",
		mcpgo.WithDescription("List commits in a repository branch. Use to walk history to find a good base or diagnose what changed."),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Bitbucket project key")),
		mcpgo.WithString("repo", mcpgo.Required(), mcpgo.Description("Repository slug")),
		mcpgo.WithString("until", mcpgo.Description("Return commits reachable from this ref or commit (defaults to default branch)")),
		mcpgo.WithString("since", mcpgo.Description("Exclude commits reachable from this ref or commit")),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of results (default 25)")),
	)
	return Spec{Tool: tool, Handler: func(c Clients) server.ToolHandlerFunc {
		svc := commitservice.NewService(c.OpenAPI)
		return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			project, _ := req.RequireString("project")
			repo, _ := req.RequireString("repo")
			commits, err := svc.List(ctx,
				commitservice.RepositoryRef{ProjectKey: project, Slug: repo},
				commitservice.ListOptions{
					Since: req.GetString("since", ""),
					Until: req.GetString("until", ""),
					Limit: req.GetInt("limit", 25),
				},
			)
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("list_commits failed", err), nil
			}
			return resultJSON(commits)
		}
	}}
}

func specGetCommit() Spec {
	tool := mcpgo.NewTool("get_commit",
		mcpgo.WithDescription("Get details of a specific commit including author, message, and timestamp."),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Bitbucket project key")),
		mcpgo.WithString("repo", mcpgo.Required(), mcpgo.Description("Repository slug")),
		mcpgo.WithString("commit_id", mcpgo.Required(), mcpgo.Description("Commit SHA")),
	)
	return Spec{Tool: tool, Handler: func(c Clients) server.ToolHandlerFunc {
		svc := commitservice.NewService(c.OpenAPI)
		return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			project, _ := req.RequireString("project")
			repo, _ := req.RequireString("repo")
			commitID, _ := req.RequireString("commit_id")
			commit, err := svc.Get(ctx,
				commitservice.RepositoryRef{ProjectKey: project, Slug: repo},
				commitID,
			)
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("get_commit failed", err), nil
			}
			return resultJSON(commit)
		}
	}}
}

func specCompareRefs() Spec {
	tool := mcpgo.NewTool("compare_refs",
		mcpgo.WithDescription("List commits between two refs. Returns the commits reachable from 'to' but not from 'from'."),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Bitbucket project key")),
		mcpgo.WithString("repo", mcpgo.Required(), mcpgo.Description("Repository slug")),
		mcpgo.WithString("from", mcpgo.Required(), mcpgo.Description("Base ref or commit (older side of comparison)")),
		mcpgo.WithString("to", mcpgo.Required(), mcpgo.Description("Target ref or commit (newer side of comparison)")),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of commits to return (default 25)")),
	)
	return Spec{Tool: tool, Handler: func(c Clients) server.ToolHandlerFunc {
		svc := commitservice.NewService(c.OpenAPI)
		return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			project, _ := req.RequireString("project")
			repo, _ := req.RequireString("repo")
			from, _ := req.RequireString("from")
			to, _ := req.RequireString("to")
			commits, err := svc.Compare(ctx,
				commitservice.RepositoryRef{ProjectKey: project, Slug: repo},
				commitservice.CompareOptions{
					From:  from,
					To:    to,
					Limit: req.GetInt("limit", 25),
				},
			)
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("compare_refs failed", err), nil
			}
			return resultJSON(commits)
		}
	}}
}
