package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type fakeManager struct {
	setupStatus        SetupStatus
	setupInitErr       error
	appInstallEvents   []ProgressEvent
	appHealthEvents    []ProgressEvent
	modelInstallEvents []ProgressEvent
}

func (f *fakeManager) SetupInit(_ context.Context, _ string, _ string, _ string, _ string) error {
	return f.setupInitErr
}

func (f *fakeManager) SetupStatus(_ context.Context) (SetupStatus, error) {
	return f.setupStatus, nil
}

func (f *fakeManager) AppInstall(_ context.Context, _ string, _ string, _ string) <-chan ProgressEvent {
	return eventsToChan(f.appInstallEvents)
}

func (f *fakeManager) AppStart(_ context.Context, _ string) <-chan ProgressEvent {
	return eventsToChan(nil)
}

func (f *fakeManager) AppStop(_ context.Context, _ string) <-chan ProgressEvent {
	return eventsToChan(nil)
}

func (f *fakeManager) AppHealth(_ context.Context, _ string, _ string, _ time.Duration, _ time.Duration) <-chan ProgressEvent {
	return eventsToChan(f.appHealthEvents)
}

func (f *fakeManager) AppList(_ context.Context) ([]App, error) {
	return nil, nil
}

func (f *fakeManager) ModelInstall(_ context.Context, _ string) <-chan ProgressEvent {
	return eventsToChan(f.modelInstallEvents)
}

func (f *fakeManager) ModelHealth(_ context.Context, _ string, _ time.Duration, _ time.Duration) <-chan ProgressEvent {
	return eventsToChan(nil)
}

func (f *fakeManager) ModelList(_ context.Context) ([]Model, error) {
	return nil, nil
}

func eventsToChan(events []ProgressEvent) <-chan ProgressEvent {
	ch := make(chan ProgressEvent, len(events))
	for _, event := range events {
		ch <- event
	}
	close(ch)
	return ch
}

func runCLI(t *testing.T, args []string, manager Manager) (int, string, string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(args, manager, &stdout, &stderr)
	return exitCode, stdout.String(), stderr.String()
}

func decodeJSONLines(t *testing.T, output string) []ProgressEvent {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	events := make([]ProgressEvent, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var event ProgressEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("failed to decode JSON line %q: %v", line, err)
		}
		events = append(events, event)
	}
	return events
}

func TestSetupInitMissingArgs(t *testing.T) {
	exitCode, _, _ := runCLI(t, []string{"setup", "init", "--json"}, &fakeManager{})
	if exitCode != ExitInvalidUsage {
		t.Fatalf("expected exit code %d, got %d", ExitInvalidUsage, exitCode)
	}
}

func TestAppInstallJSONOutput(t *testing.T) {
	manager := &fakeManager{
		appInstallEvents: []ProgressEvent{
			{Type: "log", Message: "installing"},
			{Type: "success", Message: "installed"},
		},
	}
	exitCode, stdout, _ := runCLI(t, []string{"app", "install", "ollama-cpu", "--json"}, manager)
	if exitCode != ExitSuccess {
		t.Fatalf("expected exit code %d, got %d", ExitSuccess, exitCode)
	}
	events := decodeJSONLines(t, stdout)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "log" || events[1].Type != "success" {
		t.Fatalf("unexpected event sequence: %+v", events)
	}
}

func TestAppHealthTimeoutJSON(t *testing.T) {
	manager := &fakeManager{
		appHealthEvents: []ProgressEvent{
			{Type: "error", Message: "timeout", Code: "health_timeout"},
		},
	}
	exitCode, stdout, _ := runCLI(t, []string{"app", "health", "openwebui", "--http", "http://localhost:3001", "--json"}, manager)
	if exitCode != ExitRuntimeError {
		t.Fatalf("expected exit code %d, got %d", ExitRuntimeError, exitCode)
	}
	events := decodeJSONLines(t, stdout)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "error" || events[0].Code != "health_timeout" {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

func TestModelInstallProgressJSON(t *testing.T) {
	manager := &fakeManager{
		modelInstallEvents: []ProgressEvent{
			{Type: "progress", Message: "downloading", Percent: 10},
			{Type: "success", Message: "ready"},
		},
	}
	exitCode, stdout, _ := runCLI(t, []string{"model", "install", "gemma3:270m", "--json"}, manager)
	if exitCode != ExitSuccess {
		t.Fatalf("expected exit code %d, got %d", ExitSuccess, exitCode)
	}
	events := decodeJSONLines(t, stdout)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Percent != 10 {
		t.Fatalf("expected percent 10, got %d", events[0].Percent)
	}
}

func TestSetupStatusJSON(t *testing.T) {
	manager := &fakeManager{
		setupStatus: SetupStatus{
			Complete: true,
			NodeName: "Test Node",
		},
	}
	exitCode, stdout, _ := runCLI(t, []string{"setup", "status", "--json"}, manager)
	if exitCode != ExitSuccess {
		t.Fatalf("expected exit code %d, got %d", ExitSuccess, exitCode)
	}
	events := decodeJSONLines(t, stdout)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "result" {
		t.Fatalf("expected result event, got %s", events[0].Type)
	}
	data, ok := events[0].Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", events[0].Data)
	}
	if data["complete"] != true {
		t.Fatalf("expected complete true, got %v", data["complete"])
	}
}
