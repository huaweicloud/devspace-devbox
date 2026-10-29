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

// authCmd 实现 `devbox auth`。
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: i18n.T(i18n.Msg.Auth.AuthShort),
}

// loginCmd 实现 `devbox auth login`。
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: i18n.T(i18n.Msg.Auth.LoginShort),
	Long:  i18n.T(i18n.Msg.Auth.LoginLong),
	Args:  cobra.NoArgs,
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		apiKey, _ := cmd.Flags().GetString("api-key")

		// 1. 显式 --api-key：校验后存储。
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

		// 2. 复用已有凭据（对齐 devbridge：环境变量 > keyring/配置文件），先校验。
		if cred := auth.ReadValidAPIKey(); cred != nil && cred.APIKey != "" {
			if err := auth.VerifyAPIKey(cmd.Context(), config.AuthCheckDomain, cred.APIKey); err != nil {
				return auth.LoginVerifyError(i18n.T(i18n.Msg.Auth.LoginVerifyFailed), err)
			}
			fmt.Println(i18n.T(i18n.Msg.Auth.LoginReused))
			return nil
		}

		// 3. 浏览器登录（登录页与校验地址均取构建期注入的配置）。
		return auth.BrowserLogin(cmd.Context())
	}),
}

// logoutCmd 实现 `devbox auth logout`。
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

// statusCmd 实现 `devbox auth status`。
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
