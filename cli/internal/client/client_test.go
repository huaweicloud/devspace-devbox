package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api"
	"github.com/huaweicloud/devspace-devbox/cli/internal/client"
)

// mux builds a small fake DevBox API covering the responses exercised by the
// client tests. Every handler asserts the auth header when expectedKey is set.
func mux(t *testing.T, expectedKey string) *http.ServeMux {
	t.Helper()
	m := http.NewServeMux()
	auth := func(r *http.Request) {
		if expectedKey != "" && r.Header.Get("X-API-Key") != expectedKey {
			t.Errorf("missing or wrong X-API-Key header")
		}
	}

	m.HandleFunc("/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		auth(r)
		switch r.Method {
		case http.MethodGet:
			q := r.URL.Query()
			if q.Get("metadata") != "user=abc&app=prod" {
				t.Errorf("unexpected metadata: %q", q.Get("metadata"))
			}
			if q.Get("state") != "running,paused" {
				t.Errorf("unexpected state: %q", q.Get("state"))
			}
			if q.Get("limit") != "5" {
				t.Errorf("unexpected limit: %q", q.Get("limit"))
			}
			w.Header().Set("X-Next-Token", "token-123")
			w.Header().Set("X-Total-Running", "7")
			writeJSON(t, w, http.StatusOK, []map[string]any{sandboxJSON("sb-1"), sandboxJSON("sb-2")})
		case http.MethodPost:
			var body api.NewSandbox
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode create body: %v", err)
			}
			if body.TemplateID != "tpl-1" {
				t.Errorf("unexpected templateID: %q", body.TemplateID)
			}
			if body.Timeout == nil || *body.Timeout != 30 {
				t.Errorf("unexpected timeout: %v", body.Timeout)
			}
			if body.EnvVars == nil || (*body.EnvVars)["FOO"] != "bar" {
				t.Errorf("unexpected envVars: %v", body.EnvVars)
			}
			writeJSON(t, w, http.StatusCreated, sandboxJSON("sb-new"))
		default:
			t.Errorf("unexpected method: %s", r.Method)
		}
	})

	m.HandleFunc("/sandboxes/sb-1/timeout", func(w http.ResponseWriter, r *http.Request) {
		auth(r)
		w.WriteHeader(http.StatusNoContent)
	})

	m.HandleFunc("/sandboxes/sb-1/refreshes", func(w http.ResponseWriter, r *http.Request) {
		auth(r)
		var body api.RefreshSandboxJSONBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode refresh body: %v", err)
		}
		if body.Duration != 600 {
			t.Errorf("unexpected duration: %d", body.Duration)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	m.HandleFunc("/sandboxes/sb-missing", func(w http.ResponseWriter, r *http.Request) {
		auth(r)
		writeJSON(t, w, http.StatusNotFound, map[string]string{"error": "sandbox_not_found", "message": "no such sandbox"})
	})

	m.HandleFunc("/templates", func(w http.ResponseWriter, r *http.Request) {
		auth(r)
		writeJSON(t, w, http.StatusOK, []map[string]any{
			{"templateID": "tpl-1", "name": "node-22", "aliases": []string{"node", "base"}, "buildID": "b-1", "cpuCount": 2, "memoryMB": 1024, "diskSizeMB": 4096},
		})
	})

	return m
}

func sandboxJSON(id string) map[string]any {
	return map[string]any{
		"templateID": "tpl-1",
		"sandboxID":  id,
		"startedAt":  "2026-01-02T15:04:05Z",
		"endAt":      "2026-01-02T15:05:05Z",
		"cpuCount":   int32(2),
		"memoryMB":   int32(1024),
		"diskSizeMB": int32(4096),
		"state":      "running",
		"lifecycle":  map[string]any{"onTimeout": "kill", "autoResume": false},
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func clientFor(t *testing.T, url, key string) *client.Client {
	t.Helper()
	c, err := client.New(url, key, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return c
}

func TestAPIKeyHeaderSent(t *testing.T) {
	srv := httptest.NewServer(mux(t, "sekrit"))
	defer srv.Close()

	states := []api.SandboxState{api.Running, api.Paused}
	_, err := clientFor(t, srv.URL, "sekrit").ListSandboxes(context.Background(), &api.ListSandboxesParams{
		Metadata: stringPtr("user=abc&app=prod"),
		State:    &states,
		Limit:    intPtr(5),
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
}

func TestListSandboxes(t *testing.T) {
	srv := httptest.NewServer(mux(t, ""))
	defer srv.Close()

	states := []api.SandboxState{api.Running, api.Paused}
	res, err := clientFor(t, srv.URL, "").ListSandboxes(context.Background(), &api.ListSandboxesParams{
		Metadata: stringPtr("user=abc&app=prod"),
		State:    &states,
		Limit:    intPtr(5),
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(res.Sandboxes) != 2 {
		t.Fatalf("expected 2 sandboxes, got %d", len(res.Sandboxes))
	}
	if res.NextToken != "token-123" || res.TotalRunning != 7 {
		t.Errorf("unexpected pagination meta: token=%q total=%d", res.NextToken, res.TotalRunning)
	}
	if res.Sandboxes[0].SandboxID != "sb-1" {
		t.Errorf("unexpected first sandbox: %s", res.Sandboxes[0].SandboxID)
	}
}

func TestCreateSandbox(t *testing.T) {
	srv := httptest.NewServer(mux(t, ""))
	defer srv.Close()

	envs := map[string]string{"FOO": "bar"}
	sb, err := clientFor(t, srv.URL, "").CreateSandbox(context.Background(), api.CreateSandboxJSONRequestBody(api.NewSandbox{
		TemplateID: "tpl-1",
		Timeout:    int32Ptr(30),
		EnvVars:    &envs,
	}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sb.SandboxID != "sb-new" {
		t.Fatalf("unexpected sandbox id: %s", sb.SandboxID)
	}
}

func TestSetSandboxTimeoutNoContent(t *testing.T) {
	srv := httptest.NewServer(mux(t, ""))
	defer srv.Close()

	if err := clientFor(t, srv.URL, "").SetSandboxTimeout(context.Background(), "sb-1", api.SetSandboxTimeoutJSONRequestBody(api.TimeoutOption{Timeout: int32Ptr(60)})); err != nil {
		t.Fatalf("set timeout: %v", err)
	}
}

func TestRefreshSandboxNoContent(t *testing.T) {
	srv := httptest.NewServer(mux(t, ""))
	defer srv.Close()

	if err := clientFor(t, srv.URL, "").RefreshSandbox(context.Background(), "sb-1", api.RefreshSandboxJSONRequestBody(api.RefreshSandboxJSONBody{Duration: 600})); err != nil {
		t.Fatalf("refresh: %v", err)
	}
}

func TestListTemplates(t *testing.T) {
	srv := httptest.NewServer(mux(t, ""))
	defer srv.Close()

	tpls, err := clientFor(t, srv.URL, "").ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	if len(tpls) != 1 {
		t.Fatalf("expected 1 template, got %d", len(tpls))
	}
	if tpls[0].TemplateID != "tpl-1" || tpls[0].Name != "node-22" || tpls[0].BuildID != "b-1" {
		t.Errorf("unexpected template: %+v", tpls[0])
	}
}

func TestDeleteSandboxErrorDecoded(t *testing.T) {
	srv := httptest.NewServer(mux(t, ""))
	defer srv.Close()

	err := clientFor(t, srv.URL, "").DeleteSandbox(context.Background(), "sb-missing")
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "no such sandbox") {
		t.Errorf("error should carry the API message, got: %s", msg)
	}
	if !strings.Contains(msg, "404") {
		t.Errorf("error should carry the status code, got: %s", msg)
	}
}

func TestInvalidServerURL(t *testing.T) {
	if _, err := client.New("not-a-url", "", nil); err == nil {
		t.Fatal("expected an error for a relative URL")
	}
}

func stringPtr(s string) *string { return &s }

func int32Ptr(i int32) *int32 { return &i }

func intPtr(i int) *int { return &i }
