package auth

import (
	"encoding/base64"
	"fmt"

	"github.com/huaweicloud/devspace-devbox/cli/internal/config"

	"github.com/zalando/go-keyring"

	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
)

// keyringUser is the account slot used inside the OS keyring.
const keyringUser = "default"

// StoreCredential persists the API key. The OS keyring is preferred; when it
// is unavailable the key is stored in the devbox config file as a fallback.
// An optional userInfo is always stored in the config file.
func StoreCredential(cred *Credential, userInfo *UserInfo) error {
	if cred == nil || cred.APIKey == "" {
		return fmt.Errorf("%s", i18n.T(i18n.Msg.Client.NothingToStore))
	}

	blob := base64.StdEncoding.EncodeToString([]byte(cred.APIKey))
	if err := keyring.Set(CredentialName, keyringUser, blob); err != nil {
		// Keyring unavailable: fall back to the config file.
		if err := config.Set(config.KeyAPIKey, cred.APIKey); err != nil {
			return fmt.Errorf(i18n.T(i18n.Msg.Client.StoreCredentialFailed), err)
		}
	} else {
		// Keyring succeeded: drop any stale copy from the config file.
		_ = config.Unset(config.KeyAPIKey)
	}

	if userInfo != nil {
		// Account metadata lives in the config file regardless of keyring.
		_ = config.Set(config.KeyUserInfo, userInfoToMap(userInfo))
	}
	return nil
}

// LoadCredential reads the stored API key, preferring the keyring over the
// config file. It returns an error when no credential exists.
func LoadCredential() (*Credential, error) {
	if blob, err := keyring.Get(CredentialName, keyringUser); err == nil && blob != "" {
		if data, err := base64.StdEncoding.DecodeString(blob); err == nil {
			return &Credential{APIKey: string(data)}, nil
		}
	}
	cfg, err := config.LoadAll()
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.Msg.Client.LoadCredentialFailed), err)
	}
	if key := config.Key(cfg, config.KeyAPIKey); key != "" {
		return &Credential{APIKey: key}, nil
	}
	return nil, fmt.Errorf("%s", i18n.T(i18n.Msg.Client.NoCredential))
}

// LoadUserInfo reads the optional account metadata from the config file.
func LoadUserInfo() (*UserInfo, error) {
	cfg, err := config.LoadAll()
	if err != nil {
		return nil, err
	}
	return UserInfoFromMap(cfg), nil
}

// DeleteCredential removes the API key from both the keyring and the config
// file. Failing to find it is not an error.
func DeleteCredential() error {
	_ = keyring.Delete(CredentialName, keyringUser)
	if err := config.Unset(config.KeyAPIKey); err != nil {
		return err
	}
	_ = config.Unset(config.KeyUserInfo)
	return nil
}

// userInfoToMap renders userInfo for JSON persistence.
func userInfoToMap(ui *UserInfo) map[string]any {
	return map[string]any{
		"user_name": ui.UserName,
		"user_id":   ui.UserID,
	}
}

// UserInfoFromMap parses the persisted account metadata.
func UserInfoFromMap(m map[string]any) *UserInfo {
	if m == nil {
		return nil
	}
	raw, ok := m[config.KeyUserInfo].(map[string]any)
	if !ok {
		return nil
	}
	ui := &UserInfo{}
	if v, ok := raw["user_name"].(string); ok {
		ui.UserName = v
	}
	if v, ok := raw["user_id"].(string); ok {
		ui.UserID = v
	}
	if ui.UserName == "" && ui.UserID == "" {
		return nil
	}
	return ui
}
