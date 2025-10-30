package auth0

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/auth0/auth0-cli/internal/buildinfo"
	"github.com/auth0/go-auth0/management"

	"github.com/auth0/auth0-cli/internal/utils"
)

const (
	quickstartsMetaURL            = "https://auth0.com/docs/meta/quickstarts"
	quickstartsOrg                = "auth0-samples"
	quickstartsDefaultCallbackURL = "https://YOUR_APP/callback"
)

type Quickstarts []Quickstart

type Quickstart struct {
	Name                 string `json:"name"`
	AppType              string `json:"appType"`
	URL                  string `json:"url"`
	Logo                 string `json:"logo"`
	DownloadLink         string `json:"downloadLink"`
	DownloadInstructions string `json:"downloadInstructions"`
}

func (q Quickstart) SamplePath(downloadPath string) (string, error) {
	query, err := url.ParseQuery(q.DownloadLink)
	if err != nil {
		return "", err
	}

	return path.Join(downloadPath, query.Get("path")), nil
}

func (q Quickstart) Download(ctx context.Context, downloadPath string, client *management.Client) error {
	quickstartEndpoint := fmt.Sprintf("https://auth0.com%s", q.DownloadLink)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, quickstartEndpoint, nil)
	if err != nil {
		return err
	}

	params := request.URL.Query()
	params.Add("org", quickstartsOrg)
	params.Add("client_id", client.GetClientID())

	// Callback URL, if not set, it will just take the default one.
	callbackURL := quickstartsDefaultCallbackURL
	if list := client.GetCallbacks(); len(list) > 0 {
		callbackURL = list[0]
	}
	params.Add("callback_url", callbackURL)

	request.URL.RawQuery = params.Encode()
	request.Header.Set("Content-Type", "application/json")

	userAgent := "Auth0 CLI" // Set User-Agent header using the standard CLI format.
	request.Header.Set("User-Agent", fmt.Sprintf("%v/%v", userAgent, strings.TrimPrefix(buildinfo.Version, "v")))

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status %d, got %d", http.StatusOK, response.StatusCode)
	}

	// Check if we're getting a zip file or HTML response
	contentType := response.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(contentType, "application/zip") && !strings.Contains(contentType, "application/octet-stream") {
		return fmt.Errorf("expected zip file but got content-type: %s. The quickstart endpoint may have returned an error page", contentType)
	}

	tmpFile, err := os.CreateTemp("", "auth0-quickstart*.zip")
	if err != nil {
		return err
	}

	_, err = io.Copy(tmpFile, response.Body)
	if err != nil {
		return err
	}

	if err = tmpFile.Close(); err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if err = os.RemoveAll(downloadPath); err != nil {
		return err
	}

	if err = utils.Unzip(tmpFile.Name(), downloadPath); err != nil {
		return fmt.Errorf("failed to unzip file: %w", err)
	}

	return nil
}

func GetQuickstarts(ctx context.Context) (Quickstarts, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, quickstartsMetaURL, nil)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"failed to fetch quickstarts metadata, response has status code: %d",
			response.StatusCode,
		)
	}

	var quickstarts Quickstarts
	if err := json.NewDecoder(response.Body).Decode(&quickstarts); err != nil {
		return nil, fmt.Errorf("failed to decode quickstarts metadata response: %w", err)
	}

	return quickstarts, nil
}

func (q Quickstarts) FindByStack(stack string) (Quickstart, error) {
	for _, quickstart := range q {
		if quickstart.Name == stack {
			return quickstart, nil
		}
	}

	return Quickstart{}, fmt.Errorf("failed to find any quickstarts for stack: %q", stack)
}

func (q Quickstarts) FilterByType(qsType string) (Quickstarts, error) {
	var filteredQuickstarts []Quickstart
	for _, quickstart := range q {
		if quickstart.AppType == qsType {
			filteredQuickstarts = append(filteredQuickstarts, quickstart)
		}
	}

	if len(filteredQuickstarts) == 0 {
		return nil, fmt.Errorf("failed to find any quickstarts for type: %q", qsType)
	}

	return filteredQuickstarts, nil
}

func (q Quickstarts) Stacks() []string {
	var stacks []string

	for _, qs := range q {
		stacks = append(stacks, qs.Name)
	}

	return stacks
}
