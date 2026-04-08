package githubrelease

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/vriesdemichael/bitbucket-server-cli/internal/domain/errors"
	"github.com/vriesdemichael/bitbucket-server-cli/internal/transport/network"
)

const defaultBaseURL = "https://api.github.com"

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

type Client struct {
	baseURL   string
	http      *http.Client
	userAgent string
}

func NewClient(baseURL string, httpClient *http.Client, userAgent string) *Client {
	resolvedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if resolvedBaseURL == "" {
		resolvedBaseURL = defaultBaseURL
	}

	if httpClient == nil {
		transport, err := network.NewSafeTransport(network.TLSOptions{})
		if err != nil {
			transport = &network.SafeTransport{}
		}

		httpClient = &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		}
	}

	return &Client{
		baseURL:   resolvedBaseURL,
		http:      httpClient,
		userAgent: strings.TrimSpace(userAgent),
	}
}

func (client *Client) Latest(ctx context.Context, owner, repo string) (Release, error) {
	if client == nil || client.http == nil {
		return Release{}, apperrors.New(apperrors.KindInternal, "release client is not configured", nil)
	}

	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return Release{}, apperrors.New(apperrors.KindValidation, "release repository owner and name are required", nil)
	}

	requestURL := fmt.Sprintf("%s/repos/%s/%s/releases/latest", client.baseURL, owner, repo)

	var release Release
	if err := client.do(ctx, http.MethodGet, requestURL, &release); err != nil {
		return Release{}, err
	}

	return release, nil
}

func (client *Client) Download(ctx context.Context, assetURL string) ([]byte, error) {
	if client == nil || client.http == nil {
		return nil, apperrors.New(apperrors.KindInternal, "release client is not configured", nil)
	}

	resolvedURL := strings.TrimSpace(assetURL)
	if resolvedURL == "" {
		return nil, apperrors.New(apperrors.KindValidation, "asset URL is required", nil)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, resolvedURL, nil)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "failed to build release download request", err)
	}
	request.Header.Set("Accept", "application/octet-stream")
	if client.userAgent != "" {
		request.Header.Set("User-Agent", client.userAgent)
	}

	response, err := client.http.Do(request)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to download release asset", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, mapHTTPError(response.StatusCode, "failed to download release asset")
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, apperrors.New(apperrors.KindTransient, "failed to read release asset", err)
	}

	return body, nil
}

func (client *Client) do(ctx context.Context, method, requestURL string, out any) error {
	request, err := http.NewRequestWithContext(ctx, method, requestURL, nil)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "failed to build release metadata request", err)
	}
	request.Header.Set("Accept", "application/json")
	if client.userAgent != "" {
		request.Header.Set("User-Agent", client.userAgent)
	}

	response, err := client.http.Do(request)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to fetch release metadata", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return mapHTTPError(response.StatusCode, "failed to fetch release metadata")
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return apperrors.New(apperrors.KindTransient, "failed to read release metadata", err)
	}

	if err := decodeJSON(body, out); err != nil {
		return err
	}

	return nil
}

func decodeJSON(body []byte, out any) error {
	decoder := jsonDecoder(bytes.NewReader(body))
	if err := decoder.Decode(out); err != nil {
		return apperrors.New(apperrors.KindPermanent, "failed to decode release metadata", err)
	}
	return nil
}

var jsonDecoder = func(reader io.Reader) interface{ Decode(any) error } {
	return json.NewDecoder(reader)
}

func mapHTTPError(statusCode int, message string) error {
	switch {
	case statusCode == http.StatusNotFound:
		return apperrors.New(apperrors.KindNotFound, message, nil)
	case statusCode == http.StatusTooManyRequests || statusCode >= 500:
		return apperrors.New(apperrors.KindTransient, message, nil)
	default:
		return apperrors.New(apperrors.KindPermanent, message, nil)
	}
}
