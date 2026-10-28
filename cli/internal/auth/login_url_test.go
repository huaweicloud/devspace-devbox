package auth

import (
	"strings"
	"testing"
)

func TestLoginPageURL(t *testing.T) {
	got := loginPageURL("https://devstation.connect.huaweicloud.com/", 12345)
	for _, want := range []string{
		"https://devstation.connect.huaweicloud.com/space/devbridge/redirect",
		"origin=devbox",
		"callback=12345",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("loginPageURL = %q, want to contain %q", got, want)
		}
	}
	if !strings.Contains(got, "language=") {
		t.Fatalf("loginPageURL = %q, want language param", got)
	}
}

func TestSystemLang(t *testing.T) {
	lang := systemLang()
	if lang != "zh-cn" && lang != "en-us" {
		t.Fatalf("systemLang = %q, want zh-cn or en-us", lang)
	}
}

func TestReadValidAPIKey(t *testing.T) {
	t.Setenv("HW_DEVBOX_API_KEY", "env-key")
	if cred := ReadValidAPIKey(); cred == nil || cred.APIKey != "env-key" {
		t.Fatalf("expected env key, got %+v", cred)
	}

	SetOverrideAPIKey("override-key")
	if cred := ReadValidAPIKey(); cred == nil || cred.APIKey != "override-key" {
		t.Fatalf("expected override key to win, got %+v", cred)
	}
	SetOverrideAPIKey("")
}
