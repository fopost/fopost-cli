package cmd

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	fopost "github.com/fopost/fopost-go"
)

// apiError builds the *fopost.Error the SDK would produce for a status, by
// running a real response through the SDK rather than hand-rolling the struct.
func apiError(t *testing.T, status int, body string, headers map[string]string) error {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for key, value := range headers {
			w.Header().Set(key, value)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	client, err := fopost.New("fp_test", fopost.WithBaseURL(server.URL), fopost.WithMaxRetries(1))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Workspaces.List(context.Background())
	if err == nil {
		t.Fatalf("status %d did not produce an error", status)
	}
	return err
}

func TestExitCodeMapsEveryHandledStatus(t *testing.T) {
	cases := []struct {
		status int
		want   int
	}{
		{http.StatusBadRequest, ExitValidation},
		{http.StatusUnauthorized, ExitAuth},
		{http.StatusPaymentRequired, ExitPaymentRequired},
		{http.StatusForbidden, ExitForbidden},
		{http.StatusNotFound, ExitNotFound},
		{http.StatusUnprocessableEntity, ExitValidation},
		{http.StatusTooManyRequests, ExitRateLimited},
		{http.StatusInternalServerError, ExitServer},
		{http.StatusBadGateway, ExitServer},
		{http.StatusConflict, ExitError},
	}
	for _, tc := range cases {
		err := apiError(t, tc.status, `{"error":"boom","message":"it broke"}`, nil)
		if got := ExitCode(err); got != tc.want {
			t.Errorf("ExitCode(%d) = %d, want %d", tc.status, got, tc.want)
		}
	}
}

func TestExitCodeForNonAPIErrors(t *testing.T) {
	if got := ExitCode(nil); got != ExitOK {
		t.Fatalf("ExitCode(nil) = %d, want %d", got, ExitOK)
	}
	if got := ExitCode(usageErrorf("bad flag")); got != ExitUsage {
		t.Fatalf("ExitCode(usage) = %d, want %d", got, ExitUsage)
	}
	if got := ExitCode(errors.New("something else")); got != ExitError {
		t.Fatalf("ExitCode(plain) = %d, want %d", got, ExitError)
	}
	if got := ExitCode(errors.New("dial tcp 127.0.0.1:1: connection refused")); got != ExitNetwork {
		t.Fatalf("ExitCode(transport) = %d, want %d", got, ExitNetwork)
	}
}

func TestExplainTellsAPaymentRequiredUserWhereToUpgrade(t *testing.T) {
	err := apiError(t, http.StatusPaymentRequired,
		`{"error":"subscription_required","message":"Your plan does not include this.","upgrade_url":"https://fopost.com/billing"}`, nil)
	explained := Explain(err)
	if !strings.Contains(explained, "Your plan does not include this.") {
		t.Fatalf("Explain lost the message: %q", explained)
	}
	if !strings.Contains(explained, "https://fopost.com/billing") {
		t.Fatalf("Explain did not print the upgrade URL: %q", explained)
	}
}

func TestExplainOfAPaymentRequiredWithoutAnUpgradeURLStillPointsSomewhere(t *testing.T) {
	err := apiError(t, http.StatusPaymentRequired, `{"error":"subscription_required","message":"No active plan."}`, nil)
	if !strings.Contains(Explain(err), "https://fopost.com/pricing") {
		t.Fatalf("Explain = %q, want a pricing link", Explain(err))
	}
}

func TestExplainTellsARateLimitedUserWhenToRetry(t *testing.T) {
	err := apiError(t, http.StatusTooManyRequests,
		`{"error":"rate_limited","message":"Too many requests."}`,
		map[string]string{"Retry-After": "42"})
	explained := Explain(err)
	if !strings.Contains(explained, "42s") {
		t.Fatalf("Explain = %q, want the Retry-After wait", explained)
	}

	err = apiError(t, http.StatusTooManyRequests, `{"error":"rate_limited","message":"Too many requests."}`,
		map[string]string{"X-RateLimit-Reset": itoa(int(time.Now().Add(90 * time.Second).Unix()))})
	if !strings.Contains(Explain(err), "resets in") {
		t.Fatalf("Explain = %q, want the reset window", Explain(err))
	}
}

func TestExplainTellsAnUnauthorizedUserToLogIn(t *testing.T) {
	err := apiError(t, http.StatusUnauthorized, `{"error":"unauthorized","message":"Invalid API key."}`, nil)
	if !strings.Contains(Explain(err), "fopost auth login") {
		t.Fatalf("Explain = %q, want the login hint", Explain(err))
	}
}

func TestExplainDropsTheSDKPrefixOnAPlainError(t *testing.T) {
	if got := Explain(errors.New("fopost: an API key is required")); strings.HasPrefix(got, "fopost: ") {
		t.Fatalf("Explain = %q, want the prefix stripped", got)
	}
}
