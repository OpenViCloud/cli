package output

import (
	"fmt"
	"net/http"

	"github.com/OpenViCloud/cli/api"
)

// knownCodes are the error codes the CLI explains in its own words. Any other code still reaches the user
// with the server's title, so a code added to the API later is never lost.
var knownCodes = map[string]string{
	"NOT_A_MEMBER":            "your account is not a member of any tenant",
	"TENANT_SUSPENDED":        "your tenant is suspended: you can read your resources but not change them; contact OpenViCloud support",
	"TENANT_PENDING_APPROVAL": "your tenant is waiting for approval: you can change resources once OpenViCloud approves it",
}

// Explain is the error a command returns for an API answer that was not a success.
func Explain(problem *api.Problem) error {
	sentence, known := knownCodes[problem.Code]
	if known {
		return fmt.Errorf("%s (%s)", sentence, problem.Code)
	}
	if problem.Status == http.StatusUnauthorized && problem.SentApiToken {
		return fmt.Errorf("the API refused the token in OPENVICLOUD_TOKEN: it was deleted, has expired, or is mistyped; create a new one on the portal")
	}
	if problem.Status == http.StatusUnauthorized {
		return fmt.Errorf("the API did not accept your session: run `openvicloud auth login`")
	}
	if problem.Code == "" {
		return fmt.Errorf("the API answered HTTP %d: %s", problem.Status, problem.Title)
	}
	return fmt.Errorf("%s (%s)", problem.Title, problem.Code)
}
