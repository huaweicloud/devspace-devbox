// Package config resolves the CLI configuration for the DevBox Sandbox
// Manager.
//
// The server URL and login URL are injected at build time (see
// [DefaultServer] and [LoginURL]) and are not configurable at runtime.
//
// The API key is resolved in order of priority:
//
//  1. command-line flags (bound by the caller, see cmd)
//  2. environment variables (DEVBOX_API_KEY)
//  3. the config file ($XDG_CONFIG_HOME/devbox/config.json or
//     $HOME/.devbox/config.json)
//
// Resolution is backed by github.com/spf13/viper, the de-facto configuration
// library in the cobra ecosystem. Persistence uses encoding/json on the
// config file so that only user-set values are written.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"

	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
)

// DefaultServer is the DevBox test environment server URL. It is a var (not
// const) so builds can bake a different value in at build time via the
// Makefile: -ldflags "-X .../internal/config.DefaultServer=...". It is
// intentionally not configurable at runtime.
var DefaultServer = "https://devbox-test.developer.myhuaweicloud.com"

// AuthCheckDomain is the domain of the API key verification endpoint
// ({AuthCheckDomain}/v1/api-keys/check). Build-time injectable, not
// runtime-configurable.
var AuthCheckDomain = "https://bridge.developer.myhuaweicloud.com"

// ApikeyPagePath is the API key management page path on the login portal
// ({LoginURL}{ApikeyPagePath}), shown in login hints when no browser is
// available. The page distinguishes products via the origin parameter.
const ApikeyPagePath = "/space/devbox/apikey"

// LoginURL is the DevBox login page URL. Like [DefaultServer], it is a var so
// builds can bake it via -ldflags "-X .../internal/config.LoginURL=...".
// It is intentionally not configurable at runtime.
var LoginURL = "https://devstation.connect.huaweicloud.com"

const (
	// Config keys.
	KeyAPIKey   = "api_key"
	KeyUserInfo = "user_info"

	// Environment variables used as override sources.
	EnvAPIKey = "HW_DEVBOX_API_KEY"

	configFileName = "config.json"
)

// New returns a viper instance resolving flags-independent values: env vars
// and the config file. Load the config file with [Resolve].
func New() (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigName(strings.TrimSuffix(configFileName, filepath.Ext(configFileName)))
	v.SetConfigType("json")

	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	v.AddConfigPath(dir)

	if err := v.BindEnv(KeyAPIKey, EnvAPIKey); err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.Msg.Config.BindEnvFailed), KeyAPIKey, err)
	}

	v.SetDefault(KeyAPIKey, "")

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf(i18n.T(i18n.Msg.Config.ReadConfigFileFailed), err)
		}
	}
	return v, nil
}

// Resolved is the effective configuration for one invocation.
type Resolved struct {
	APIKey string
}

// Resolve overlays the resolved configuration with the command-line flag
// value when provided.
func (r *Resolved) Resolve(flagAPIKey string) {
	if flagAPIKey != "" {
		r.APIKey = flagAPIKey
	}
}

// Load reads the effective configuration, applying env + file + defaults and
// then the given flag override.
func Load(flagAPIKey string) (*Resolved, error) {
	v, err := New()
	if err != nil {
		return nil, err
	}
	r := &Resolved{
		APIKey: v.GetString(KeyAPIKey),
	}
	r.Resolve(flagAPIKey)
	return r, nil
}

// Path returns the absolute path of the config file (for display).
func Path() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// ConfigDir returns the devbox config directory, honouring XDG on Linux and
// falling back to $HOME/.devbox.
func ConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "devbox"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf(i18n.T(i18n.Msg.Config.ResolveHomeFailed), err)
	}
	return filepath.Join(home, ".devbox"), nil
}

// Set persists a single key in the config file.
func Set(key string, value any) error {
	values, err := readPersisted()
	if err != nil {
		return err
	}
	values[key] = value
	return writePersisted(values)
}

// Unset removes a key from the config file.
func Unset(key string) error {
	values, err := readPersisted()
	if err != nil {
		return err
	}
	delete(values, key)
	return writePersisted(values)
}

// LoadAll returns only the values currently stored in the config file.
func LoadAll() (map[string]any, error) {
	return readPersisted()
}

// Key returns the string value (if any) stored at key in m.
func Key(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// readPersisted loads only the values currently stored in the config file.
func readPersisted() (map[string]any, error) {
	values := make(map[string]any)
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return values, nil
		}
		return nil, fmt.Errorf(i18n.T(i18n.Msg.Config.ReadConfigFileFailed), err)
	}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.Msg.Config.ParseConfigFileFailed), err)
	}
	return values, nil
}

// writePersisted stores `values` as compact, sorted JSON in the config file.
func writePersisted(values map[string]any) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf(i18n.T(i18n.Msg.Config.CreateConfigDirFailed), err)
	}

	// Sort keys for deterministic diffs and drop empty values.
	keys := make([]string, 0, len(values))
	for k := range values {
		if values[k] != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		val, err := json.Marshal(values[k])
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.Msg.Config.EncodeConfigFailed), err)
		}
		parts = append(parts, jsonKey(k)+`:`+string(val))
	}

	path, err := Path()
	if err != nil {
		return err
	}
	data := []byte("{" + strings.Join(parts, ",") + "}\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf(i18n.T(i18n.Msg.Config.WriteConfigFileFailed), err)
	}
	return nil
}

func jsonKey(k string) string {
	data, err := json.Marshal(k)
	if err != nil {
		return `""`
	}
	return string(data)
}

// AllowedKeys returns the keys accepted by `devbox config set <key>`.
func AllowedKeys() []string {
	return []string{KeyAPIKey}
}
