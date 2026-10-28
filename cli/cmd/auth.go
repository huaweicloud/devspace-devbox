package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/huaweicloud/devspace-devbox/cli/internal/auth"
	"github.com/huaweicloud/devspace-devbox/cli/internal/config"

	"github.com/spf13/cobra"
)

// authCmd 实现 `devbox auth`。
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate to the DevBox sandbox manager",
}

// loginCmd 实现 `devbox auth login`。
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "log in to CLI",
	Long: `Log in to the DevBox sandbox manager.

The API key is resolved in this order:
  1. the --api-key flag (verified and stored)
  2. a key already stored in the OS keyring or config file (verified)
  3. an interactive browser login against the configured login page`,
	Args: cobra.NoArgs,
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		apiKey, _ := cmd.Flags().GetString("api-key")

		// 1. 显式 --api-key：校验后存储。
		if apiKey != "" {
			if err := auth.VerifyAPIKey(cmd.Context(), config.AuthCheckDomain, apiKey); err != nil {
				return auth.LoginVerifyError("登录失败：", err)
			}
			if err := auth.StoreCredential(&auth.Credential{APIKey: apiKey}, nil); err != nil {
				return err
			}
			fmt.Println("登录成功")
			return nil
		}

		// 2. 复用已有凭据（对齐 devbridge：环境变量 > keyring/配置文件），先校验。
		if cred := auth.ReadValidAPIKey(); cred != nil && cred.APIKey != "" {
			if err := auth.VerifyAPIKey(cmd.Context(), config.AuthCheckDomain, cred.APIKey); err != nil {
				return auth.LoginVerifyError("登录失败：本地 API Key 校验未通过：", err)
			}
			fmt.Println("登录成功（已复用本地保存的 API Key）")
			return nil
		}

		// 3. 浏览器登录（登录页与校验地址均取构建期注入的配置）。
		return auth.BrowserLogin(cmd.Context())
	}),
}

// logoutCmd 实现 `devbox auth logout`。
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "log out of CLI",
	Args:  cobra.NoArgs,
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		if err := auth.DeleteCredential(); err != nil {
			return err
		}
		fmt.Println("已退出登录")
		return nil
	}),
}

// statusCmd 实现 `devbox auth status`。
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current login state",
	Args:  cobra.NoArgs,
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		cred, err := auth.LoadCredential()
		if err != nil {
			fmt.Println("未登录")
			return nil
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		if err := auth.VerifyAPIKey(ctx, config.AuthCheckDomain, cred.APIKey); err != nil {
			if auth.IsUnauthorized(err) {
				fmt.Println("未登录（本地 API Key 已失效，请运行 devbox auth login）")
				return nil
			}
			fmt.Printf("已登录，但校验失败：%v\n", err)
			return nil
		}
		fmt.Println("已登录")
		if ui, err := auth.LoadUserInfo(); err == nil && ui != nil && ui.UserName != "" {
			fmt.Printf("用户名: %s\n", ui.UserName)
		}
		return nil
	}),
}

func init() {
	loginCmd.Flags().String("api-key", "",
		"API key to verify and store (env "+config.EnvAPIKey+")")

	authCmd.AddCommand(loginCmd, logoutCmd, statusCmd)
}
