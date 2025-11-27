package runtime

import (
	"strings"
	"testing"

	"treeos/internal/naming"
)

func TestProjectNameCandidates(t *testing.T) {
	app := &App{Name: "OpenWebUI", Path: "/opt/ontree/apps/OpenWebUI"}
	candidates := projectNameCandidates(app)

	expected := []string{"openwebui", "ontree-openwebui"}
	if len(candidates) == 0 {
		t.Fatalf("expected candidates to be populated")
	}

	lower := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			t.Fatalf("unexpected empty candidate: %#v", candidates)
		}
		lower[strings.ToLower(candidate)] = struct{}{}
	}

	for _, want := range expected {
		if _, ok := lower[want]; !ok {
			t.Fatalf("expected candidate %q to be present in %#v", want, candidates)
		}
	}
}

func TestContainerMatchesProject(t *testing.T) {
	app := &App{Name: "OpenWebUI", Path: "/opt/ontree/apps/OpenWebUI"}
	candidates := projectNameCandidates(app)
	project := naming.GetComposeProjectName(naming.GetAppIdentifier(app.Path))
	cont := dockerContainer{
		Names: []string{"ontree-openwebui-app-1"},
		Labels: map[string]string{
			"com.docker.compose.project": project,
			"com.docker.compose.service": "app",
		},
	}

	if !containerMatchesProject(cont, candidates) {
		t.Fatalf("expected container to match project")
	}

	cont = dockerContainer{
		Names:  []string{"/ontree-openwebui-app-1"},
		Labels: map[string]string{},
	}
	if !containerMatchesProject(cont, candidates) {
		t.Fatalf("expected container with prefixed name to match")
	}

	cont = dockerContainer{
		Names:  []string{"/other-app-1"},
		Labels: map[string]string{},
	}
	if containerMatchesProject(cont, candidates) {
		t.Fatalf("did not expect unrelated container to match")
	}
}

func TestGetContainerStatus(t *testing.T) {
	app := &App{Name: "Demo", Path: "/opt/ontree/apps/Demo"}
	candidates := projectNameCandidates(app)

	running := dockerContainer{Names: []string{"ontree-demo-web-1"}, State: "running", Labels: map[string]string{"com.docker.compose.project": candidates[0]}}
	exited := dockerContainer{Names: []string{"ontree-demo-worker-1"}, State: "exited", Labels: map[string]string{"com.docker.compose.project": candidates[0]}}

	status := (&Client{}).getContainerStatus(app, []dockerContainer{running})
	if status != "running" {
		t.Fatalf("expected running status, got %s", status)
	}

	status = (&Client{}).getContainerStatus(app, []dockerContainer{running, exited})
	if status != "partial" {
		t.Fatalf("expected partial status, got %s", status)
	}

	status = (&Client{}).getContainerStatus(app, []dockerContainer{exited})
	if status != "exited" {
		t.Fatalf("expected exited status, got %s", status)
	}
}
