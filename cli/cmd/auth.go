package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/huaweicloud/devspace-devbox/cli/internal/auth"
	"github.com/huaweicloud/devspace-devbox/cli/internal/config"
	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"

	"github.com/spf13/cobra"
)

// authCmd implements `devbox auth`.
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: i18n.T(i18n.Msg.Auth.AuthShort),
}

// loginCmd implements `devbox auth login`.
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: i18n.T(i18n.Msg.Auth.LoginShort),
	Long:  i18n.T(i18n.Msg.Auth.LoginLong),
	Args:  cobra.NoArgs,
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		apiKey, _ := cmd.Flags().GetString("api-key")

		// 1. explicit --api-key: verified and stored.
		if apiKey != "" {
			if err := auth.VerifyAPIKey(cmd.Context(), config.AuthCheckDomain, apiKey); err != nil {
				return auth.LoginVerifyError(i18n.T(i18n.Msg.Auth.LoginFailed), err)
			}
			if err := auth.StoreCredential(&auth.Credential{APIKey: apiKey}, nil); err != nil {
				return err
			}
			fmt.Println(i18n.T(i18n.Msg.Auth.LoginSuccess))
			return nil
		}

		// 2. reuse existing credentials (aligned with devbridge: env var > keyring/config file), verify first.
		if cred := auth.ReadValidAPIKey(); cred != nil && cred.APIKey != "" {
			if err := auth.VerifyAPIKey(cmd.Context(), config.AuthCheckDomain, cred.APIKey); err != nil {
				return auth.LoginVerifyError(i18n.T(i18n.Msg.Auth.LoginVerifyFailed), err)
			}
			fmt.Println(i18n.T(i18n.Msg.Auth.LoginReused))
			return nil
		}

		// 3. browser login (both the login page and the check address come from build-time config).
		return auth.BrowserLogin(cmd.Context())
	}),
}

// logoutCmd implements `devbox auth logout`.
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: i18n.T(i18n.Msg.Auth.LogoutShort),
	Args:  cobra.NoArgs,
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		if err := auth.DeleteCredential(); err != nil {
			return err
		}
		fmt.Println(i18n.T(i18n.Msg.Auth.LogoutSuccess))
		return nil
	}),
}

// statusCmd implements `devbox auth status`.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: i18n.T(i18n.Msg.Auth.StatusShort),
	Args:  cobra.NoArgs,
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		cred, err := auth.LoadCredential()
		if err != nil {
			fmt.Println(i18n.T(i18n.Msg.Auth.NotLoggedIn))
			return nil
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		if err := auth.VerifyAPIKey(ctx, config.AuthCheckDomain, cred.APIKey); err != nil {
			if auth.IsUnauthorized(err) {
				fmt.Println(i18n.T(i18n.Msg.Auth.CredInvalidHint))
				return nil
			}
			fmt.Printf(i18n.T(i18n.Msg.Auth.StatusVerifyFailed)+"\n", err)
			return nil
		}
		fmt.Println(i18n.T(i18n.Msg.Auth.LoggedIn))
		if ui, err := auth.LoadUserInfo(); err == nil && ui != nil && ui.UserName != "" {
			fmt.Printf(i18n.T(i18n.Msg.Auth.UserName)+"\n", ui.UserName)
		}
		return nil
	}),
}

func init() {
	loginCmd.Flags().String("api-key", "",
		i18n.T(i18n.Msg.Auth.APIKeyFlag)+" ("+config.EnvAPIKey+")")

	authCmd.AddCommand(loginCmd, logoutCmd, statusCmd)
}
