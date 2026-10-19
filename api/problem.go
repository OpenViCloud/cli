package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Problem is the error body of every /v1 route: { status, title, code } (CLAUDE.md, error response shape).
type Problem struct {
	Status int    `json:"status"`
	Title  string `json:"title"`
	Code   string `json:"code"`
	// SentApiToken: the refused request carried an API token (ovc_), not a login session, so a 401 means the token
	// was refused.
	SentApiToken bool `json:"-"`
}

func (p *Problem) Error() string {
	if p.Code == "" {
		return fmt.Sprintf("HTTP %d: %s", p.Status, p.Title)
	}
	return fmt.Sprintf("HTTP %d %s: %s", p.Status, p.Code, p.Title)
}

// ProblemOf reads the error body of a non-2xx answer. A body that is not a problem (a proxy's HTML page,
// an empty 401) still yields the HTTP status.
func ProblemOf(response *http.Response, body []byte) *Problem {
	var problem Problem
	err := json.Unmarshal(body, &problem)
	if err != nil {
		problem = Problem{}
	}
	if problem.Status == 0 {
		problem.Status = response.StatusCode
	}
	if problem.Title == "" {
		problem.Title = http.StatusText(response.StatusCode)
	}
	if response.Request != nil {
		problem.SentApiToken = strings.HasPrefix(response.Request.Header.Get("Authorization"), "Bearer ovc_")
	}
	return &problem
}

// New is a client of the API at server that sends the customer's access token and the CLI version.
func New(server string, accessToken string, cliVersion string, httpClient *http.Client) (*ClientWithResponses, error) {
	addHeaders := func(ctx context.Context, request *http.Request) error {
		request.Header.Set("Authorization", "Bearer "+accessToken)
		request.Header.Set("User-Agent", "openvicloud-cli/"+cliVersion)
		return nil
	}
	return NewClientWithResponses(server, WithHTTPClient(httpClient), WithRequestEditorFn(addHeaders))
}
