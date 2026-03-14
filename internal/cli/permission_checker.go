package cli

import (
	"context"
	"fmt"
	"strings"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/openapi"
	openapigenerated "github.com/vriesdemichael/bitbucket-server-cli/internal/openapi/generated"
)

// PermissionChecker provides pre-flight permission checking during dry-run.
type PermissionChecker struct {
	client *openapigenerated.ClientWithResponses
	cache  map[string]error
}

// NewPermissionChecker creates a new PermissionChecker.
func NewPermissionChecker(client *openapigenerated.ClientWithResponses) *PermissionChecker {
	return &PermissionChecker{
		client: client,
		cache:  make(map[string]error),
	}
}

// CheckRepoPermission verifies if the caller has the specified permission on a repository.
func (p *PermissionChecker) CheckRepoPermission(ctx context.Context, projectKey, repoSlug string, permission openapigenerated.GetRepositories1ParamsPermission) error {
	cacheKey := fmt.Sprintf("repo:%s/%s:%s", projectKey, repoSlug, permission)
	if err, ok := p.cache[cacheKey]; ok {
		return err
	}

	limit := float32(1)
	params := &openapigenerated.GetRepositories1Params{
		Projectkey: &projectKey,
		Permission: &permission,
		Limit:      &limit,
	}

	resp, err := p.client.GetRepositories1WithResponse(ctx, params)
	if err != nil {
		p.cache[cacheKey] = err
		return err
	}
	if resp.StatusCode() >= 400 {
		err := openapi.MapStatusError(resp.StatusCode(), resp.Body)
		p.cache[cacheKey] = err
		return err
	}

	if resp.ApplicationjsonCharsetUTF8200 == nil || resp.ApplicationjsonCharsetUTF8200.Values == nil || len(*resp.ApplicationjsonCharsetUTF8200.Values) == 0 {
		err := apperrors.New(apperrors.KindAuthorization, fmt.Sprintf("insufficient permission: %s required on repository %s/%s", permission, projectKey, repoSlug), nil)
		p.cache[cacheKey] = err
		return err
	}

	found := false
	for _, repo := range *resp.ApplicationjsonCharsetUTF8200.Values {
		if repo.Slug == nil || !strings.EqualFold(strings.TrimSpace(*repo.Slug), repoSlug) {
			continue
		}
		if repo.Project != nil && strings.EqualFold(strings.TrimSpace(repo.Project.Key), projectKey) {
			found = true
			break
		}
	}
	if !found {
		err := apperrors.New(apperrors.KindAuthorization, fmt.Sprintf("insufficient permission: %s required on repository %s/%s", permission, projectKey, repoSlug), nil)
		p.cache[cacheKey] = err
		return err
	}

	p.cache[cacheKey] = nil
	return nil
}

// CheckProjectWrite verifies if the caller has PROJECT_WRITE on a project.
func (p *PermissionChecker) CheckProjectWrite(ctx context.Context, projectKey string) error {
	cacheKey := fmt.Sprintf("project:%s:PROJECT_WRITE", projectKey)
	if err, ok := p.cache[cacheKey]; ok {
		return err
	}

	// First resolve the project name
	projResp, err := p.client.GetProjectWithResponse(ctx, projectKey)
	if err != nil {
		p.cache[cacheKey] = err
		return err
	}
	if projResp.StatusCode() >= 400 {
		err := openapi.MapStatusError(projResp.StatusCode(), projResp.Body)
		p.cache[cacheKey] = err
		return err
	}
	if projResp.ApplicationjsonCharsetUTF8200 == nil || projResp.ApplicationjsonCharsetUTF8200.Name == nil {
		err := apperrors.New(apperrors.KindInternal, fmt.Sprintf("failed to resolve project name for key %s", projectKey), nil)
		p.cache[cacheKey] = err
		return err
	}

	name := *projResp.ApplicationjsonCharsetUTF8200.Name
	perm := "PROJECT_WRITE"
	limit := float32(1)
	params := &openapigenerated.GetProjectsParams{
		Name:       &name,
		Permission: &perm,
		Limit:      &limit,
	}

	resp, err := p.client.GetProjectsWithResponse(ctx, params)
	if err != nil {
		p.cache[cacheKey] = err
		return err
	}
	if resp.StatusCode() >= 400 {
		err := openapi.MapStatusError(resp.StatusCode(), resp.Body)
		p.cache[cacheKey] = err
		return err
	}

	if resp.ApplicationjsonCharsetUTF8200 == nil || resp.ApplicationjsonCharsetUTF8200.Values == nil || len(*resp.ApplicationjsonCharsetUTF8200.Values) == 0 {
		err := apperrors.New(apperrors.KindAuthorization, fmt.Sprintf("insufficient permission: PROJECT_WRITE required on project %s", projectKey), nil)
		p.cache[cacheKey] = err
		return err
	}

	// Verify the key matches just in case
	found := false
	for _, proj := range *resp.ApplicationjsonCharsetUTF8200.Values {
		if proj.Key != nil && strings.EqualFold(*proj.Key, projectKey) {
			found = true
			break
		}
	}

	if !found {
		err := apperrors.New(apperrors.KindAuthorization, fmt.Sprintf("insufficient permission: PROJECT_WRITE required on project %s", projectKey), nil)
		p.cache[cacheKey] = err
		return err
	}

	p.cache[cacheKey] = nil
	return nil
}

// CheckProjectAdmin verifies if the caller has PROJECT_ADMIN on a project.
func (p *PermissionChecker) CheckProjectAdmin(ctx context.Context, projectKey string) error {
	cacheKey := fmt.Sprintf("project:%s:PROJECT_ADMIN", projectKey)
	if err, ok := p.cache[cacheKey]; ok {
		return err
	}

	limit := float32(1)
	params := &openapigenerated.GetUsersWithAnyPermission1Params{
		Limit: &limit,
	}

	resp, err := p.client.GetUsersWithAnyPermission1WithResponse(ctx, projectKey, params)
	if err != nil {
		p.cache[cacheKey] = err
		return err
	}
	if resp.StatusCode() >= 400 {
		err := openapi.MapStatusError(resp.StatusCode(), resp.Body)
		p.cache[cacheKey] = err
		return err
	}

	p.cache[cacheKey] = nil
	return nil
}

// CheckProjectCreate verifies if the caller can create projects by intentionally
// sending an invalid create payload. A 400 means the request reached validation,
// so the caller is authorized to use the create-project endpoint. 401/403 mean
// authentication/authorization failure before validation.
func (p *PermissionChecker) CheckProjectCreate(ctx context.Context) error {
	cacheKey := "global:PROJECT_CREATE"
	if err, ok := p.cache[cacheKey]; ok {
		return err
	}

	resp, err := p.client.CreateProjectWithResponse(ctx, openapigenerated.RestProject{})
	if err != nil {
		p.cache[cacheKey] = err
		return err
	}

	switch resp.StatusCode() {
	case 400:
		p.cache[cacheKey] = nil
		return nil
	case 401, 403:
		err := openapi.MapStatusError(resp.StatusCode(), resp.Body)
		p.cache[cacheKey] = err
		return err
	default:
		err := apperrors.New(apperrors.KindPermanent, fmt.Sprintf("project create permission probe returned unexpected status %d", resp.StatusCode()), nil)
		p.cache[cacheKey] = err
		return err
	}
}
