package repository

import (
	"context"
	"fmt"
	"strconv"

	"github.com/vriesdemichael/bitbucket-server-cli/internal/transport/httpclient"
)

type Repository struct {
	ProjectKey string `json:"project_key"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Public     bool   `json:"public"`
}

type Service struct {
	client *httpclient.Client
}

func NewService(client *httpclient.Client) *Service {
	return &Service{client: client}
}

func (service *Service) List(ctx context.Context, limit int) ([]Repository, error) {
	return service.listPaged(ctx, "/rest/api/1.0/repos", limit)
}

func (service *Service) ListByProject(ctx context.Context, projectKey string, limit int) ([]Repository, error) {
	if projectKey == "" {
		return nil, fmt.Errorf("project key is required")
	}

	return service.listPaged(ctx, "/rest/api/1.0/projects/"+projectKey+"/repos", limit)
}

func (service *Service) listPaged(ctx context.Context, path string, limit int) ([]Repository, error) {
	if limit <= 0 {
		limit = 25
	}

	results := []Repository{}
	start := 0

	for {
		var response pagedRepoResponse
		err := service.client.GetJSON(
			ctx,
			path,
			map[string]string{"limit": strconv.Itoa(limit), "start": strconv.Itoa(start)},
			&response,
		)
		if err != nil {
			return nil, err
		}

		for _, value := range response.Values {
			results = append(results, Repository{
				ProjectKey: value.Project.Key,
				Slug:       value.Slug,
				Name:       value.Name,
				Public:     value.Public,
			})
		}

		if response.IsLastPage {
			break
		}

		start = response.NextPageStart
	}

	return results, nil
}

type pagedRepoResponse struct {
	Values        []repoValue `json:"values"`
	IsLastPage    bool        `json:"isLastPage"`
	NextPageStart int         `json:"nextPageStart"`
}

type repoValue struct {
	Slug    string      `json:"slug"`
	Name    string      `json:"name"`
	Public  bool        `json:"public"`
	Project projectInfo `json:"project"`
}

type projectInfo struct {
	Key string `json:"key"`
}
