package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	fopost "github.com/fopost/fopost-go"
)

// Exit codes. 0 and 1 carry their usual meaning; the rest let a script branch
// on what went wrong without parsing the message.
const (
	ExitOK              = 0
	ExitError           = 1
	ExitUsage           = 2
	ExitAuth            = 3
	ExitPaymentRequired = 4
	ExitForbidden       = 5
	ExitNotFound        = 6
	ExitRateLimited     = 7
	ExitValidation      = 8
	ExitServer          = 9
	ExitNetwork         = 10
)

// UsageError is a mistake in the invocation rather than a failure of the call.
type UsageError struct{ msg string }

func usageErrorf(format string, args ...any) error {
	return &UsageError{msg: fmt.Sprintf(format, args...)}
}

func (e *UsageError) Error() string { return e.msg }

// ExitCode maps an error onto the process exit code it should produce.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var usage *UsageError
	if errors.As(err, &usage) {
		return ExitUsage
	}
	if apiErr, ok := fopost.APIError(err); ok {
		switch {
		case apiErr.Status == http.StatusUnauthorized:
			return ExitAuth
		case apiErr.Status == http.StatusPaymentRequired:
			return ExitPaymentRequired
		case apiErr.Status == http.StatusForbidden:
			return ExitForbidden
		case apiErr.Status == http.StatusNotFound:
			return ExitNotFound
		case apiErr.Status == http.StatusTooManyRequests:
			return ExitRateLimited
		case apiErr.Status == http.StatusBadRequest, apiErr.Status == http.StatusUnprocessableEntity:
			return ExitValidation
		case apiErr.Status >= 500:
			return ExitServer
		}
		return ExitError
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || errors.Is(err, io.ErrUnexpectedEOF) {
		return ExitNetwork
	}
	// The SDK wraps transport failures without a status; those are network.
	if isTransportError(err) {
		return ExitNetwork
	}
	return ExitError
}

func isTransportError(err error) bool {
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr interface{ Unwrap() error }
	_ = urlErr
	return strings.Contains(err.Error(), "dial tcp") ||
		strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "no such host")
}

// Explain renders an error as the one line a user should read, plus the extra
// line an actionable status deserves.
func Explain(err error) string {
	if err == nil {
		return ""
	}
	apiErr, ok := fopost.APIError(err)
	if !ok {
		return strings.TrimPrefix(err.Error(), "fopost: ")
	}

	message := apiErr.Message
	if message == "" {
		message = http.StatusText(apiErr.Status)
	}

	switch apiErr.Status {
	case http.StatusUnauthorized:
		return message + "\nRun `fopost auth login` with a key from Settings → API Keys."
	case http.StatusPaymentRequired:
		if url := apiErr.UpgradeURL(); url != "" {
			return message + "\nUpgrade your plan to continue: " + url
		}
		return message + "\nUpgrade your plan to continue: https://fopost.com/pricing"
	case http.StatusForbidden:
		return message + "\nThe key is valid but lacks the scope or workspace access this needs."
	case http.StatusNotFound:
		return message
	case http.StatusTooManyRequests:
		return message + "\n" + retryAdvice(apiErr)
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return message
	}
	if apiErr.Status >= 500 {
		return message + "\nThe API is having trouble. The request was retried; try again shortly."
	}
	return message
}

func retryAdvice(apiErr *fopost.Error) string {
	if apiErr.RetryAfter > 0 {
		return fmt.Sprintf("Rate limited. Retry in %s.", roundWait(apiErr.RetryAfter))
	}
	if !apiErr.RateLimit.Reset.IsZero() {
		if wait := time.Until(apiErr.RateLimit.Reset); wait > 0 {
			return fmt.Sprintf("Rate limited. The window resets in %s.", roundWait(wait))
		}
	}
	return "Rate limited. Retry in a minute."
}

func roundWait(d time.Duration) time.Duration {
	if d < time.Second {
		return time.Second
	}
	return d.Round(time.Second)
}
