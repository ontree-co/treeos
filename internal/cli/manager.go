package cli

import (
	"context"
	"time"
)

// Manager abstracts core operations for the CLI.
type Manager interface {
	SetupInit(ctx context.Context, username, password, nodeName, nodeIcon string) error
	SetupStatus(ctx context.Context) (SetupStatus, error)

	AppInstall(ctx context.Context, appID, version, envPath string) <-chan ProgressEvent
	AppStart(ctx context.Context, appID string) <-chan ProgressEvent
	AppStop(ctx context.Context, appID string) <-chan ProgressEvent
	AppHealth(ctx context.Context, appID, httpURL string, timeout, interval time.Duration) <-chan ProgressEvent
	AppList(ctx context.Context) ([]App, error)

	ModelInstall(ctx context.Context, model string) <-chan ProgressEvent
	ModelHealth(ctx context.Context, model string, timeout, interval time.Duration) <-chan ProgressEvent
	ModelList(ctx context.Context) ([]Model, error)
}
