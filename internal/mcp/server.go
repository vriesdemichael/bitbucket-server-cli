package mcp

import (
	"fmt"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/config"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/openapi"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/transport/httpclient"
)

// Clients bundles the two HTTP client variants and the resolved base URL consumed by the service layer.
type Clients struct {
	HTTP    *httpclient.Client
	OpenAPI *openapigenerated.ClientWithResponses
	BaseURL string // normalised Bitbucket base URL (no trailing slash)
}

// ClientsFromConfig builds both client types from a resolved AppConfig.
func ClientsFromConfig(cfg config.AppConfig) (Clients, error) {
	httpClient := httpclient.NewFromConfig(cfg)
	openAPIClient, err := openapi.NewClientWithResponsesFromConfig(cfg)
	if err != nil {
		return Clients{}, fmt.Errorf("failed to create openapi client: %w", err)
	}
	return Clients{
		HTTP:    httpClient,
		OpenAPI: openAPIClient,
		BaseURL: strings.TrimRight(cfg.BitbucketURL, "/"),
	}, nil
}

// Spec pairs a tool definition with a handler factory so that metadata
// can be listed without a live Bitbucket connection.
type Spec struct {
	Tool    mcpgo.Tool
	Handler func(Clients) server.ToolHandlerFunc
}

// AllSpecs returns the full catalog of MCP tool specifications in stable order.
func AllSpecs() []Spec {
	return []Spec{
		// Pull request group
		specGetPullRequest(),
		specListPullRequests(),
		specCreatePullRequest(),
		specListPRComments(),
		specAddPRComment(),
		specListPRTasks(),
		specSubmitPRReview(),
		specMergePullRequest(),
		// Repository group
		specSearchRepositories(),
		specGetRepositoryCloneInfo(),
		// Branch / ref group
		specListBranches(),
		specResolveRef(),
		// Tag group
		specListTags(),
		specCreateTag(),
		// Build / quality group
		specGetBuildStatus(),
		specSetBuildStatus(),
		specListRequiredBuilds(),
		// Commit group
		specListCommits(),
		specGetCommit(),
		specCompareRefs(),
	}
}

// NewServer creates a configured MCPServer with optional tool filtering.
// allow is a list of tool names to expose exclusively (empty = all).
// exclude is a list of tool names to suppress.
// When allow is non-empty it takes precedence; exclude is still applied afterwards.
func NewServer(name, version string, clients Clients, allow, exclude []string) *server.MCPServer {
	s := server.NewMCPServer(name, version, server.WithToolCapabilities(false))

	allowSet := toSet(allow)
	excludeSet := toSet(exclude)

	for _, spec := range AllSpecs() {
		toolName := spec.Tool.Name
		if len(allowSet) > 0 && !allowSet[toolName] {
			continue
		}
		if excludeSet[toolName] {
			continue
		}
		s.AddTool(spec.Tool, spec.Handler(clients))
	}

	return s
}

// toSet converts a string slice into a presence map, trimming whitespace.
func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		if t := strings.TrimSpace(item); t != "" {
			m[t] = true
		}
	}
	return m
}

// resultJSON serialises data as a JSON tool result.
// Serialisation errors are surfaced as error results rather than Go errors
// because tool handlers report operational errors through the result value.
func resultJSON(data any) (*mcpgo.CallToolResult, error) {
	result, serErr := mcpgo.NewToolResultJSON(data)
	if serErr != nil {
		return mcpgo.NewToolResultErrorFromErr("failed to serialize result", serErr), nil
	}
	return result, nil
}
