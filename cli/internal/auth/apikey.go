package auth

import (
	"os"
	"sync/atomic"
)

// envAPIKey is the environment variable holding the API key.
const envAPIKey = "HW_DEVBOX_API_KEY"

// overrideAPIKey is written once at startup (flag binding) and read from
// request paths; atomic storage keeps it race-free.
var overrideAPIKey atomic.Value

// SetOverrideAPIKey sets the command-line API key so subsequent API calls use
// it directly.
func SetOverrideAPIKey(key string) {
	overrideAPIKey.Store(key)
}

// overrideKey returns the command-line API key override ("" when unset).
func overrideKey() string {
	if v := overrideAPIKey.Load(); v != nil {
		return v.(string)
	}
	return ""
}

// ReadValidAPIKey returns the effective API key. The resolution order is:
// command-line override first, then the HW_DEVBOX_API_KEY
// environment variable, then the stored credential (keyring preferred, config
// file as the fallback). It returns nil when no key is available.
func ReadValidAPIKey() *Credential {
	if key := overrideKey(); key != "" {
		return &Credential{APIKey: key}
	}
	if cred := loadFromEnv(); cred != nil {
		return cred
	}
	if cred, err := LoadCredential(); err == nil && cred != nil {
		return cred
	}
	return nil
}

func loadFromEnv() *Credential {
	apiKey := os.Getenv(envAPIKey)
	if apiKey == "" {
		return nil
	}
	return &Credential{APIKey: apiKey}
}
