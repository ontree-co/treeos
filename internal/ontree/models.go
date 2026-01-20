package ontree

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ModelInstall pulls a model from the running Ollama container.
func (m *Manager) ModelInstall(ctx context.Context, model string) <-chan ProgressEvent {
	ch := make(chan ProgressEvent, 1)
	go func() {
		defer close(ch)
		if model == "" {
			ch <- ProgressEvent{Type: "error", Message: "model name required", Code: "invalid_model"}
			return
		}

		containerName, err := m.findOllamaContainer(ctx)
		if err != nil {
			ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "ollama_not_running"}
			return
		}

		cmd := m.execCommand(ctx, "docker", "exec", containerName, "ollama", "pull", model)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "exec_failed"}
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "exec_failed"}
			return
		}

		if err := cmd.Start(); err != nil {
			ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "exec_failed"}
			return
		}

		readOutput := func(reader readCloser) {
			defer reader.Close()
			scanner := bufio.NewScanner(reader)
			for scanner.Scan() {
				ch <- ProgressEvent{Type: "progress", Message: scanner.Text()}
			}
		}

		go readOutput(stdout)
		go readOutput(stderr)

		if err := cmd.Wait(); err != nil {
			ch <- ProgressEvent{Type: "error", Message: err.Error(), Code: "exec_failed"}
			return
		}

		ch <- ProgressEvent{Type: "success", Message: "model installed"}
	}()
	return ch
}

// ModelHealth waits for a model to be available in Ollama.
func (m *Manager) ModelHealth(ctx context.Context, model string, timeout, interval time.Duration) <-chan ProgressEvent {
	ch := make(chan ProgressEvent, 1)
	go func() {
		defer close(ch)
		if model == "" {
			ch <- ProgressEvent{Type: "error", Message: "model name required", Code: "invalid_model"}
			return
		}
		if timeout <= 0 {
			timeout = 180 * time.Second
		}
		if interval <= 0 {
			interval = 3 * time.Second
		}

		deadline := time.Now().Add(timeout)
		for {
			select {
			case <-ctx.Done():
				ch <- ProgressEvent{Type: "error", Message: ctx.Err().Error(), Code: "context_cancelled"}
				return
			default:
			}

			models, err := m.listOllamaModels(ctx)
			if err == nil {
				for _, name := range models {
					if name == model {
						ch <- ProgressEvent{Type: "success", Message: "model ready"}
						return
					}
				}
			}

			if time.Now().After(deadline) {
				ch <- ProgressEvent{Type: "error", Message: "health check timeout", Code: "health_timeout"}
				return
			}
			time.Sleep(interval)
		}
	}()
	return ch
}

// ModelList returns available models in Ollama.
func (m *Manager) ModelList(ctx context.Context) ([]Model, error) {
	names, err := m.listOllamaModels(ctx)
	if err != nil {
		return nil, err
	}
	models := make([]Model, 0, len(names))
	for _, name := range names {
		models = append(models, Model{Name: name})
	}
	return models, nil
}

func (m *Manager) listOllamaModels(ctx context.Context) ([]string, error) {
	containerName, err := m.findOllamaContainer(ctx)
	if err != nil {
		return nil, err
	}

	cmd := m.execCommand(ctx, "docker", "exec", containerName, "ollama", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) <= 1 {
		return []string{}, nil
	}

	models := make([]string, 0, len(lines)-1)
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			models = append(models, fields[0])
		}
	}
	return models, nil
}

func (m *Manager) findOllamaContainer(ctx context.Context) (string, error) {
	cmd := m.execCommand(ctx, "docker", "ps", "--filter", "label=ontree.inference=true", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to discover Ollama container: %w", err)
	}

	containers := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(containers) == 0 || containers[0] == "" {
		return "", errors.New("no Ollama container is running")
	}

	return containers[0], nil
}
