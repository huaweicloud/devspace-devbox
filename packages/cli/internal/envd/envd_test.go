package envd_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/huaweicloud/devspace-devbox/packages/cli/internal/envd"
)

// frame builds a connect+json frame: [flags][4-byte BE length][payload].
func frame(flags byte, payload []byte) []byte {
	f := make([]byte, 5+len(payload))
	f[0] = flags
	n := len(payload)
	f[1] = byte(n >> 24)
	f[2] = byte(n >> 16)
	f[3] = byte(n >> 8)
	f[4] = byte(n)
	copy(f[5:], payload)
	return f
}

func TestTunnelPortExtraction(t *testing.T) {
	cases := map[string]string{
		"https://6vtzwk32-49983.devbox-s2.hwtunnel.com": "49983",
		"https://host.example.com:8443":                 "8443",
		"https://plain.example.com":                     "",
	}
	for domain, want := range cases {
		c, err := envd.New(domain, "sb-1", "tok", nil)
		if err != nil {
			t.Fatalf("new(%s): %v", domain, err)
		}
		if got := c.Port(); got != want {
			t.Errorf("port(%s) = %q, want %q", domain, got, want)
		}
	}
}

func TestStartPTYAndSendInput(t *testing.T) {
	var gotHeaders http.Header
	var gotStartBody []byte
	var gotInputBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/process.Process/Start", func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		body, _ := io.ReadAll(r.Body)
		gotStartBody = body
		w.Header().Set("Content-Type", "application/connect+json")
		w.WriteHeader(http.StatusOK)
		start, _ := json.Marshal(map[string]any{"event": map[string]any{"start": map[string]any{"pid": 123}}})
		data, _ := json.Marshal(map[string]any{"event": map[string]any{"data": map[string]any{"pty": "aGVsbG8="}}})
		end, _ := json.Marshal(map[string]any{"event": map[string]any{"end": map[string]any{"exitCode": 0, "exited": true, "status": "exited"}}})
		trailer := []byte(`{}`)
		_, _ = w.Write(frame(0x00, start))
		_, _ = w.Write(frame(0x00, data))
		_, _ = w.Write(frame(0x00, end))
		_, _ = w.Write(frame(0x02, trailer))
	})
	mux.HandleFunc("/process.Process/SendInput", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotInputBody = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := envd.New(srv.URL, "sb-1", "tok-123", srv.Client())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if c.Port() == "" {
		t.Fatalf("expected a tunnel port for the httptest URL")
	}

	stream, pid, err := c.StartPTY(context.Background(), 120, 30, "")
	if err != nil {
		t.Fatalf("start pty: %v", err)
	}
	defer func() { _ = stream.Close() }()

	if pid != 123 {
		t.Fatalf("unexpected pid: %d", pid)
	}

	if gotHeaders.Get("Cookie") != "relay_token=tok-123" {
		t.Errorf("unexpected cookie: %q", gotHeaders.Get("Cookie"))
	}
	if gotHeaders.Get("E2B-Sandbox-Id") != "sb-1" {
		t.Errorf("unexpected sandbox id header: %q", gotHeaders.Get("E2B-Sandbox-Id"))
	}
	if gotHeaders.Get("Connect-Protocol-Version") != "1" {
		t.Errorf("unexpected protocol version: %q", gotHeaders.Get("Connect-Protocol-Version"))
	}
	if gotHeaders.Get("Content-Type") != "application/connect+json" {
		t.Errorf("unexpected content type: %q", gotHeaders.Get("Content-Type"))
	}
	if len(gotStartBody) < 5 || gotStartBody[0] != 0x00 {
		t.Errorf("start body should be a 0x00 frame, got %d bytes", len(gotStartBody))
	}

	var startReq struct {
		Process struct {
			Cmd  string   `json:"cmd"`
			Args []string `json:"args"`
		} `json:"process"`
		Stdin bool `json:"stdin"`
		PTY   *struct {
			Size struct {
				Cols int `json:"cols"`
				Rows int `json:"rows"`
			} `json:"size"`
		} `json:"pty"`
	}
	if err := json.Unmarshal(gotStartBody[5:], &startReq); err != nil {
		t.Fatalf("decode start body: %v", err)
	}
	if startReq.Process.Cmd != "/bin/bash" || !startReq.Stdin || startReq.PTY == nil || startReq.PTY.Size.Cols != 120 || startReq.PTY.Size.Rows != 30 {
		t.Errorf("unexpected start request: %+v", startReq)
	}

	// Read remaining events: data then end.
	evData, err := stream.Next()
	if err != nil {
		t.Fatalf("next (data): %v", err)
	}
	if string(evData.Data) != "hello" {
		t.Errorf("unexpected data payload: %q", evData.Data)
	}
	evEnd, err := stream.Next()
	if err != nil {
		t.Fatalf("next (end): %v", err)
	}
	if evEnd.End == nil || evEnd.End.ExitCode != 0 || evEnd.End.Status != "exited" {
		t.Errorf("unexpected end event: %+v", evEnd.End)
	}
	if _, err := stream.Next(); err != io.EOF {
		t.Errorf("expected io.EOF after trailer, got %v", err)
	}

	if err := c.SendInput(context.Background(), 123, []byte("hi\n")); err != nil {
		t.Fatalf("send input: %v", err)
	}
	var inputReq struct {
		Process struct {
			PID uint32 `json:"pid"`
		} `json:"process"`
		Input struct {
			PTY []byte `json:"pty"`
		} `json:"input"`
	}
	if err := json.Unmarshal(gotInputBody, &inputReq); err != nil {
		t.Fatalf("decode input body: %v", err)
	}
	if inputReq.Process.PID != 123 || string(inputReq.Input.PTY) != "hi\n" {
		t.Errorf("unexpected input request: %+v", inputReq)
	}
}

func TestNewRejectsEmptyToken(t *testing.T) {
	if _, err := envd.New("https://example.com", "sb-1", "", nil); err == nil {
		t.Fatal("expected an error for an empty connect token")
	}
	if _, err := envd.New("", "sb-1", "tok", nil); err == nil {
		t.Fatal("expected an error for an empty domain")
	}
}
