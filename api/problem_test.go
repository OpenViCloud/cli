package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProblemOf_ReadsStatusTitleAndCode(t *testing.T) {
	response := &http.Response{StatusCode: http.StatusForbidden}
	body := []byte(`{"status":403,"title":"Tenant is suspended","code":"TENANT_SUSPENDED"}`)

	problem := ProblemOf(response, body)

	if problem.Status != 403 || problem.Title != "Tenant is suspended" || problem.Code != "TENANT_SUSPENDED" {
		t.Fatalf("problem = %+v", problem)
	}
}

func TestProblemOf_BodyIsNotAProblem_FallsBackToTheHttpStatus(t *testing.T) {
	response := &http.Response{StatusCode: http.StatusBadGateway}

	problem := ProblemOf(response, []byte("<html>bad gateway</html>"))

	if problem.Status != 502 || problem.Title != "Bad Gateway" || problem.Code != "" {
		t.Fatalf("problem = %+v", problem)
	}
}

func TestNew_SendsTheTokenAndTheCliVersion(t *testing.T) {
	var authorization string
	var userAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		userAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"0b6f6a8e-0000-0000-0000-000000000001","name":"acme"}`))
	}))
	defer server.Close()
	client, err := New(server.URL, "token-1", "1.2.3", server.Client())
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.TitanInfrastructureEndpointsTenantGetTenantEndpointWithResponse(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if authorization != "Bearer token-1" {
		t.Fatalf("Authorization = %q", authorization)
	}
	if userAgent != "openvicloud-cli/1.2.3" {
		t.Fatalf("User-Agent = %q", userAgent)
	}
}
