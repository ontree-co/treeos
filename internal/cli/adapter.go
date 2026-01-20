package cli

import (
	"context"
	"time"

	"github.com/ontree-co/treeos/internal/ontree"
)

// NewManagerAdapter wraps an ontree.Manager for CLI usage.
func NewManagerAdapter(manager *ontree.Manager) Manager {
	return &managerAdapter{manager: manager}
}

type managerAdapter struct {
	manager *ontree.Manager
}

func (m *managerAdapter) SetupInit(ctx context.Context, username, password, nodeName, nodeIcon string) error {
	return m.manager.SetupInit(ctx, username, password, nodeName, nodeIcon)
}

func (m *managerAdapter) SetupStatus(ctx context.Context) (SetupStatus, error) {
	status, err := m.manager.SetupStatus(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	return SetupStatus{
		Complete: status.Complete,
		NodeName: status.NodeName,
		NodeIcon: status.NodeIcon,
	}, nil
}

func (m *managerAdapter) AppInstall(ctx context.Context, appID, version, envPath string) <-chan ProgressEvent {
	return convertEvents(m.manager.AppInstall(ctx, appID, version, envPath))
}

func (m *managerAdapter) AppStart(ctx context.Context, appID string) <-chan ProgressEvent {
	return convertEvents(m.manager.AppStart(ctx, appID))
}

func (m *managerAdapter) AppStop(ctx context.Context, appID string) <-chan ProgressEvent {
	return convertEvents(m.manager.AppStop(ctx, appID))
}

func (m *managerAdapter) AppHealth(ctx context.Context, appID, httpURL string, timeout, interval time.Duration) <-chan ProgressEvent {
	return convertEvents(m.manager.AppHealth(ctx, appID, httpURL, timeout, interval))
}

func (m *managerAdapter) AppList(ctx context.Context) ([]App, error) {
	apps, err := m.manager.AppList(ctx)
	if err != nil {
		return nil, err
	}
	converted := make([]App, 0, len(apps))
	for _, app := range apps {
		converted = append(converted, App{ID: app.ID, Name: app.Name})
	}
	return converted, nil
}

func (m *managerAdapter) ModelInstall(ctx context.Context, model string) <-chan ProgressEvent {
	return convertEvents(m.manager.ModelInstall(ctx, model))
}

func (m *managerAdapter) ModelHealth(ctx context.Context, model string, timeout, interval time.Duration) <-chan ProgressEvent {
	return convertEvents(m.manager.ModelHealth(ctx, model, timeout, interval))
}

func (m *managerAdapter) ModelList(ctx context.Context) ([]Model, error) {
	models, err := m.manager.ModelList(ctx)
	if err != nil {
		return nil, err
	}
	converted := make([]Model, 0, len(models))
	for _, model := range models {
		converted = append(converted, Model{Name: model.Name})
	}
	return converted, nil
}

func convertEvents(input <-chan ontree.ProgressEvent) <-chan ProgressEvent {
	out := make(chan ProgressEvent, 1)
	go func() {
		defer close(out)
		for event := range input {
			out <- ProgressEvent{
				Type:    event.Type,
				Message: event.Message,
				Code:    event.Code,
				Percent: event.Percent,
				Data:    event.Data,
			}
		}
	}()
	return out
}
