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
//  3. the config file ($HOME/.huawei/devbox/config.json)
//
// Persistence uses encoding/json on the
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

	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
)

// DefaultServer is the DevBox test environment server URL. It is a var (not
// const) so builds can bake a different value in at build time via the
// Makefile: -ldflags "-X .../internal/config.DefaultServer=...". It is
// intentionally not configurable at runtime.
var DefaultServer = "https://devbox-test.developer.myhuaweicloud.com"

// ApikeyPagePath is the API key management page path on the login portal
// ({LoginURL}{ApikeyPagePath}), shown in login hints when no browser is
// available. The page distinguishes products via the origin parameter.
const ApikeyPagePath = "/space/devbox/apikey"

// LoginURL is the DevBox login page URL. Like [DefaultServer], it is a var so
// builds can bake it via -ldflags "-X .../internal/config.LoginURL=...".
// It is intentionally not configurable at runtime.
var LoginURL = "https://devstation.connect.huaweicloud.com"

const (
	// KeyAPIKey is the config-file key holding the API key fallback.
	KeyAPIKey = "api_key"
	// KeyUserInfo is the config-file key holding the account metadata.
	KeyUserInfo = "user_info"

	// EnvAPIKey is the environment variable overriding the stored API key.
	EnvAPIKey = "HW_DEVBOX_API_KEY"

	configFileName = "config.json"
)

// Path returns the absolute path of the config file (for display).
func Path() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// ConfigDir returns the devbox config directory (~/.huawei/devbox),
// aligned with the shared dot-directory layout.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf(i18n.T(i18n.Msg.Config.ResolveHomeFailed), err)
	}
	return filepath.Join(home, ".huawei", "devbox"), nil
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
// A missing or empty file is treated as no persisted values (an empty file is
// what writePersisted leaves after everything is removed).
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
	if len(strings.TrimSpace(string(data))) == 0 {
		return values, nil
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
		if isEmptyValue(values[k]) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	path, err := Path()
	if err != nil {
		return err
	}

	// Nothing left to persist: write an empty file instead of "{}".
	if len(keys) == 0 {
		return writeFileAtomic(path, []byte(""))
	}

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		val, err := json.Marshal(values[k])
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.Msg.Config.EncodeConfigFailed), err)
		}
		parts = append(parts, jsonKey(k)+`:`+string(val))
	}

	data := []byte("{" + strings.Join(parts, ",") + "}\n")
	return writeFileAtomic(path, data)
}

// writeFileAtomic writes data via a temp file + rename so an interrupted
// write never leaves a truncated config behind.
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf(i18n.T(i18n.Msg.Config.WriteConfigFileFailed), err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
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

// isEmptyValue reports whether v is a zero-ish value that should not be
// persisted: nil, empty string, 0 or false. Composite values (maps, slices)
// are always kept.
func isEmptyValue(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case bool:
		return !x
	case int:
		return x == 0
	case float64:
		return x == 0
	default:
		return false
	}
}
