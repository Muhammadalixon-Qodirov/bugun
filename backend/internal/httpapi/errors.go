package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"bugun/internal/errtrack"
)

// internalError logs the cause and returns a generic message, so internals
// never leak to the client.
func internalError(ctx context.Context, op string, err error) error {
	slog.ErrorContext(ctx, op, "err", err)
	errtrack.CaptureError(ctx, op, err) // no-op without SENTRY_DSN
	return errorForStatus(http.StatusInternalServerError)
}
