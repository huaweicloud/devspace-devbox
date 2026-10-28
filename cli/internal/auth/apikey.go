package auth

import "os"

// envAPIKey is the environment variable holding the API key.
const envAPIKey = "HW_DEVBOX_API_KEY"

var overrideAPIKey string

// SetOverrideAPIKey sets the command-line API key so subsequent API calls use
// it directly.
func SetOverrideAPIKey(key string) {
	overrideAPIKey = key
}

// ReadValidAPIKey returns the effective API key. The resolution order mirrors
// devbridge's: command-line override first, then the HW_DEVBOX_API_KEY
// environment variable, then the stored credential (keyring preferred, config
// file as the fallback). It returns nil when no key is available.
func ReadValidAPIKey() *Credential {
	if overrideAPIKey != "" {
		return &Credential{APIKey: overrideAPIKey}
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
