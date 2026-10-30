package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The devstation login page nests the payload under "result".
func TestStartLoginWithNestedResult(t *testing.T) {
	session, err := StartLogin("https://devstation.connect.huaweicloud.com")
	if err != nil {
		t.Fatalf("start login: %v", err)
	}
	defer session.Close()

	u, _ := url.Parse(session.URL)
	port := u.Query().Get("callback")
	body := strings.NewReader(`{"error_code":"0000","error_msg":"success","result":{"apiKey":"nested-key-9","userName":"hw067826022","userId":"u-2"}}`)
	resp, err := http.Post("http://127.0.0.1:"+port+"/", "application/json", body)
	if err != nil {
		t.Fatalf("post callback: %v", err)
	}
	_ = resp.Body.Close()

	cred, ui, err := session.Wait(context.Background())
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if cred.APIKey != "nested-key-9" {
		t.Fatalf("expected nested-key-9, got %q", cred.APIKey)
	}
	if ui == nil || ui.UserName != "hw067826022" || ui.UserID != "u-2" {
		t.Fatalf("expected user info lifted from result, got %+v", ui)
	}
}
