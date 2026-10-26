package cmd

import (
	"fmt"
	"strconv"

	"github.com/huaweicloud/devspace-devbox/packages/cli/internal/api"
	"github.com/huaweicloud/devspace-devbox/packages/cli/internal/output"

	"github.com/spf13/cobra"
)

// sandboxCmd 实现 `devbox sandbox`。
var sandboxCmd = &cobra.Command{
	// Use 里的 "sandbox|sbx" 仅用于帮助展示；命令匹配靠 Aliases。
	Use:     "sandbox|sbx",
	Aliases: []string{"sandbox", "sbx"},
	Short:   "work with sandboxes",
}

// sandboxCreateCmd 的 flag 绑定变量。
var (
	sandboxCreateTimeout    int32
	sandboxCreateMetadata   []string
	sandboxCreateEnv        []string
	sandboxCreateBuildID    string
	sandboxCreateOnTimeout  string
	sandboxCreateAutoResume bool
)

// sandboxCreateCmd 实现 `devbox sandbox create <templateID>`，对应 POST /sandboxes。
var sandboxCreateCmd = &cobra.Command{
	Use:   "create <templateID>",
	Short: "Create a sandbox from a template",
	Args:  cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		req := api.NewSandbox{TemplateID: args[0]}
		if cmd.Flags().Changed("timeout") {
			req.Timeout = pInt32(sandboxCreateTimeout)
		}
		if cmd.Flags().Changed("build-id") {
			req.BuildID = pString(sandboxCreateBuildID)
		}
		if req.Metadata, err = parseMetadata(sandboxCreateMetadata); err != nil {
			return fmt.Errorf("invalid --metadata: %w", err)
		}
		envMap, err := parseKV(sandboxCreateEnv)
		if err != nil {
			return fmt.Errorf("invalid --env: %w", err)
		}
		req.EnvVars = pStringMap(envMap)

		if cmd.Flags().Changed("lifecycle.ontimeout") {
			ot := api.SandboxLifecycleOnTimeout(sandboxCreateOnTimeout)
			if !ot.Valid() {
				return fmt.Errorf("invalid --lifecycle.ontimeout %q (supported: pause, kill)", sandboxCreateOnTimeout)
			}
			if sandboxCreateAutoResume && sandboxCreateOnTimeout != "pause" {
				return fmt.Errorf("--lifecycle.autoresume requires --lifecycle.ontimeout pause")
			}
			req.Lifecycle = &api.SandboxLifecycle{OnTimeout: ot, AutoResume: sandboxCreateAutoResume}
		} else if cmd.Flags().Changed("lifecycle.autoresume") {
			return fmt.Errorf("--lifecycle.autoresume requires --lifecycle.ontimeout pause")
		}

		sb, err := c.CreateSandbox(cmd.Context(), api.CreateSandboxJSONRequestBody(req))
		if err != nil {
			return err
		}
		return output.Print(cmd.OutOrStdout(), out(cmd), output.Spec{
			JSON: sb,
			KV: []output.KV{
				{Key: "Sandbox ID", Value: sb.SandboxID},
				{Key: "Template ID", Value: sb.TemplateID},
				{Key: "State", Value: string(sb.State)},
				{Key: "Ended At", Value: formatTimePtr(sb.EndAt)},
				{Key: "Domain", Value: output.Dash(strFromPtr(sb.Domain))},
			},
		})
	}),
}

// sandboxListCmd 的 flag 绑定变量。
var (
	sandboxListMetadata  string
	sandboxListStates    []string
	sandboxListNextToken string
	sandboxListLimit     int32
)

// sandboxListCmd 实现 `devbox sandbox list`，对应 GET /sandboxes。
var sandboxListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sandboxes",
	Args:  cobra.NoArgs,
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		params := &api.ListSandboxesParams{
			Metadata:  optString(sandboxListMetadata),
			NextToken: optString(sandboxListNextToken),
		}
		if cmd.Flags().Changed("state") {
			states := make([]api.SandboxState, 0, len(sandboxListStates))
			for _, s := range sandboxListStates {
				st := api.SandboxState(s)
				if !st.Valid() {
					return fmt.Errorf("invalid state %q (supported: running, paused)", s)
				}
				states = append(states, st)
			}
			params.State = &states
		}
		if cmd.Flags().Changed("limit") {
			params.Limit = pInt(sandboxListLimit)
		}

		res, err := c.ListSandboxes(cmd.Context(), params)
		if err != nil {
			return err
		}
		if len(res.Sandboxes) == 0 && out(cmd) == output.FormatText {
			fmt.Println("No sandboxes found")
			return nil
		}
		rows := make([][]string, 0, len(res.Sandboxes))
		for _, sb := range res.Sandboxes {
			rows = append(rows, []string{
				sb.SandboxID,
				sb.TemplateID,
				formatTime(sb.StartedAt),
				formatTimePtr(sb.EndAt),
				string(sb.State),
				strconv.Itoa(int(sb.CpuCount)),
				strconv.Itoa(int(sb.MemoryMB)),
				strconv.Itoa(int(sb.DiskSizeMB)),
				formatMetadata(sb.Metadata),
			})
		}
		if err := output.Print(cmd.OutOrStdout(), out(cmd), output.Spec{
			JSON:    res.Sandboxes,
			Headers: []string{"SANDBOX ID", "TEMPLATE ID", "STARTED AT", "END AT", "STATE", "VCPU", "RAM(MB)", "DISK(MB)", "METADATA"},
			Rows:    rows,
		}); err != nil {
			return err
		}
		if res.NextToken != "" && out(cmd) == output.FormatText {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "next token: %s (pass back with --next-token)\n", res.NextToken)
		}
		return nil
	}),
}

// sandboxInfoCmd 实现 `devbox sandbox info <sandboxID>`，对应 GET /sandboxes/{id}。
var sandboxInfoCmd = &cobra.Command{
	Use:   "info <sandboxID>",
	Short: "show information for a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		sb, err := c.GetSandbox(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		kv := []output.KV{
			{Key: "Sandbox ID", Value: sb.SandboxID},
			{Key: "Template ID", Value: sb.TemplateID},
			{Key: "State", Value: string(sb.State)},
			{Key: "Started At", Value: formatTime(sb.StartedAt)},
			{Key: "Ended At", Value: formatTimePtr(sb.EndAt)},
			{Key: "VCPU", Value: strconv.Itoa(int(sb.CpuCount))},
			{Key: "RAM(MB)", Value: strconv.Itoa(int(sb.MemoryMB))},
			{Key: "Disk(MB)", Value: strconv.Itoa(int(sb.DiskSizeMB))},
			{Key: "Lifecycle", Value: formatJSON(sb.Lifecycle)},
			{Key: "Metadata", Value: formatMetadata(sb.Metadata)},
			{Key: "Tunnel ID", Value: output.Dash(strFromPtr(sb.TunnelId))},
			{Key: "Domain", Value: output.Dash(strFromPtr(sb.Domain))},
		}
		return output.Print(cmd.OutOrStdout(), out(cmd), output.Spec{JSON: sb, KV: kv})
	}),
}

// sandboxKillCmd 实现 `devbox sandbox kill <sandboxID>`，对应 DELETE /sandboxes/{id}。
var sandboxKillCmd = &cobra.Command{
	Use:   "kill <sandboxID>",
	Short: "Kill a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := c.DeleteSandbox(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Printf("sandbox %s has been killed\n", args[0])
		return nil
	}),
}

// sandboxResumeCmd 的 flag 绑定变量。
var sandboxResumeTimeout int32

// sandboxPauseCmd 实现 `devbox sandbox pause <sandboxID>`，对应 POST /sandboxes/{id}/pause。
var sandboxPauseCmd = &cobra.Command{
	Use:   "pause <sandboxID>",
	Short: "Pause a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := c.PauseSandbox(cmd.Context(), args[0]); err != nil {
			return err
		}
		fmt.Printf("sandbox %s has been paused\n", args[0])
		return nil
	}),
}

// sandboxResumeCmd 实现 `devbox sandbox resume <sandboxID>`，对应 POST /sandboxes/{id}/resume。
var sandboxResumeCmd = &cobra.Command{
	Use:   "resume <sandboxID>",
	Short: "Resume a paused sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		sb, err := c.ResumeSandbox(cmd.Context(), args[0], api.ResumeSandboxJSONRequestBody(api.TimeoutOption{
			Timeout: pInt32(sandboxResumeTimeout),
		}))
		if err != nil {
			return err
		}
		return output.Print(cmd.OutOrStdout(), out(cmd), output.Spec{
			JSON: sb,
			KV: []output.KV{
				{Key: "Sandbox ID", Value: sb.SandboxID},
				{Key: "State", Value: string(sb.State)},
			},
		})
	}),
}

// sandboxConnectCmd 的 flag 绑定变量。
var sandboxConnectTimeout int32

// sandboxConnectCmd 实现 `devbox sandbox connect <sandboxID>`，对应 POST /sandboxes/{id}/connect。
var sandboxConnectCmd = &cobra.Command{
	Use:   "connect <sandboxID>",
	Short: "Connect to a sandbox, resuming it if paused",
	Args:  cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		sb, err := c.ConnectSandbox(cmd.Context(), args[0], api.ConnectSandboxJSONRequestBody(api.TimeoutOption{
			Timeout: pInt32(sandboxConnectTimeout),
		}))
		if err != nil {
			return err
		}
		return output.Print(cmd.OutOrStdout(), out(cmd), output.Spec{
			JSON: sb,
			KV: []output.KV{
				{Key: "Sandbox ID", Value: sb.SandboxID},
				{Key: "State", Value: string(sb.State)},
				{Key: "Ended At", Value: formatTimePtr(sb.EndAt)},
				{Key: "Domain", Value: output.Dash(strFromPtr(sb.Domain))},
			},
		})
	}),
}

// sandboxTimeoutCmd 的 flag 绑定变量。
var sandboxTimeoutValue int32

// sandboxTimeoutCmd 实现 `devbox sandbox timeout <sandboxID>`，对应 POST /sandboxes/{id}/timeout。
var sandboxTimeoutCmd = &cobra.Command{
	Use:   "timeout <sandboxID>",
	Short: "Set the time-to-live of a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := c.SetSandboxTimeout(cmd.Context(), args[0], api.SetSandboxTimeoutJSONRequestBody(api.TimeoutOption{
			Timeout: pInt32(sandboxTimeoutValue),
		})); err != nil {
			return err
		}
		fmt.Printf("sandbox %s timeout set to %ds\n", args[0], sandboxTimeoutValue)
		return nil
	}),
}

// sandboxRefreshCmd 的 flag 绑定变量。
var sandboxRefreshDuration int32

// sandboxRefreshCmd 实现 `devbox sandbox refresh <sandboxID>`，对应 POST /sandboxes/{id}/refreshes。
var sandboxRefreshCmd = &cobra.Command{
	Use:   "refresh <sandboxID>",
	Short: "Extend the lifetime of a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := c.RefreshSandbox(cmd.Context(), args[0], api.RefreshSandboxJSONRequestBody(api.RefreshSandboxJSONBody{
			Duration: sandboxRefreshDuration,
		})); err != nil {
			return err
		}
		fmt.Printf("sandbox %s lifetime extended by %ds\n", args[0], sandboxRefreshDuration)
		return nil
	}),
}

func init() {
	sandboxCreateCmd.Flags().Int32VarP(&sandboxCreateTimeout, "timeout", "t", 300,
		"Time to live for the sandbox in seconds (10-86400)")
	sandboxCreateCmd.Flags().StringArrayVarP(&sandboxCreateMetadata, "metadata", "m", nil,
		"Metadata key=value (repeatable)")
	sandboxCreateCmd.Flags().StringArrayVarP(&sandboxCreateEnv, "env", "e", nil,
		"Environment variable key=value (repeatable)")
	sandboxCreateCmd.Flags().StringVarP(&sandboxCreateBuildID, "build-id", "b", "",
		"Template build identifier")
	sandboxCreateCmd.Flags().StringVar(&sandboxCreateOnTimeout, "lifecycle.ontimeout", "",
		"Action when sandbox timeout is reached: pause or kill")
	sandboxCreateCmd.Flags().BoolVar(&sandboxCreateAutoResume, "lifecycle.autoresume", false,
		"Enable sandbox auto-resume, requires --lifecycle.ontimeout pause")

	sandboxListCmd.Flags().StringVarP(&sandboxListMetadata, "metadata", "m", "",
		`Metadata filter, e.g. "user=abc&app=prod"`)
	sandboxListCmd.Flags().StringSliceVarP(&sandboxListStates, "state", "s", nil,
		"Filter by state: running or paused (repeatable)")
	sandboxListCmd.Flags().StringVarP(&sandboxListNextToken, "next-token", "n", "",
		"Pagination cursor")
	sandboxListCmd.Flags().Int32VarP(&sandboxListLimit, "limit", "l", 0,
		"Maximum number of results (1-100)")

	sandboxResumeCmd.Flags().Int32Var(&sandboxResumeTimeout, "timeout", 300,
		"New time to live in seconds (10-86400)")

	sandboxConnectCmd.Flags().Int32Var(&sandboxConnectTimeout, "timeout", 300,
		"Time to live in seconds (10-86400)")

	sandboxTimeoutCmd.Flags().Int32VarP(&sandboxTimeoutValue, "timeout", "t", 300,
		"New time to live in seconds (10-86400)")

	sandboxRefreshCmd.Flags().Int32VarP(&sandboxRefreshDuration, "duration", "d", 300,
		"Lifetime extension in seconds (10-86400)")

	sandboxCmd.AddCommand(
		sandboxCreateCmd,
		sandboxListCmd,
		sandboxInfoCmd,
		sandboxKillCmd,
		sandboxPauseCmd,
		sandboxResumeCmd,
		sandboxConnectCmd,
		sandboxTimeoutCmd,
		sandboxRefreshCmd,
	)
}
