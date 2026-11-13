package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/huaweicloud/devspace-devbox/cli/internal/api"
	"github.com/huaweicloud/devspace-devbox/cli/internal/api/processgen"
	"github.com/huaweicloud/devspace-devbox/cli/internal/client"
	"github.com/huaweicloud/devspace-devbox/cli/internal/helpers"
	"github.com/huaweicloud/devspace-devbox/cli/internal/i18n"
	"github.com/huaweicloud/devspace-devbox/cli/internal/output"

	"github.com/spf13/cobra"
)

// sandboxCmd implements `devbox sandbox`.
var sandboxCmd = &cobra.Command{
	// The "sandbox|sbx" in Use is only for help display; matching relies on Aliases.
	Use:     "sandbox|sbx",
	Aliases: []string{"sandbox", "sbx"},
	Short:   i18n.T(i18n.Msg.Sandbox.SandboxShort),
}

// sandboxCreateCmd's flag bound variables.
var (
	sandboxCreateTimeout            int32
	sandboxCreateMetadata           []string
	sandboxCreateEnv                []string
	sandboxCreateBuildID            string
	sandboxCreateOnTimeout          string
	sandboxCreateAutoResume         bool
	sandboxCreateNetworkAllowPublic bool
	sandboxCreateDetach             bool
	sandboxCreateCwd                string
	sandboxCreateUser               string
)

// sandboxCreateCmd implements `devbox sandbox create <templateID>`, corresponding to POST /sandboxes.
// Aligned with e2b: by default the terminal opens right after creation; --detach only creates.
var sandboxCreateCmd = &cobra.Command{
	Use:     "create|cr <templateID>",
	Aliases: []string{"create", "cr"},
	Short:   i18n.T(i18n.Msg.Sandbox.CreateShort),
	Args:    cobra.ExactArgs(1),
	RunE:    runError(runSandboxCreate),
}

// runSandboxCreate validates the input, creates the sandbox and opens the
// interactive terminal unless --detach is given.
func runSandboxCreate(cmd *cobra.Command, args []string) error {
	if err := helpers.ValidateTemplateID(args[0]); err != nil {
		return err
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	req := api.NewSandbox{TemplateID: args[0]}
	if cmd.Flags().Changed("timeout") {
		if err := helpers.ValidateTimeoutSeconds(sandboxCreateTimeout); err != nil {
			return err
		}
		req.Timeout = api.PInt32(sandboxCreateTimeout)
	}
	if cmd.Flags().Changed("build-id") {
		req.BuildID = api.PString(sandboxCreateBuildID)
	}
	if req.Metadata, err = helpers.ParseMetadata(sandboxCreateMetadata); err != nil {
		return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidMetadata), err)
	}
	envMap, err := helpers.ParseKV(sandboxCreateEnv)
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidEnv), err)
	}
	// Aligned with e2b: -e only sets the terminal session's environment variables (sandbox-level envVars are not written).

	if cmd.Flags().Changed("lifecycle.ontimeout") {
		ot := api.SandboxLifecycleOnTimeout(sandboxCreateOnTimeout)
		if !ot.Valid() {
			return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidOnTimeout), sandboxCreateOnTimeout)
		}
		if sandboxCreateAutoResume && sandboxCreateOnTimeout != string(api.Pause) {
			return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.AutoResumeRequires))
		}
		req.Lifecycle = &api.SandboxLifecycle{OnTimeout: ot, AutoResume: sandboxCreateAutoResume}
	} else if cmd.Flags().Changed("lifecycle.autoresume") {
		return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.AutoResumeRequires))
	}

	// Only submitted when the user explicitly passes the flag; omitted otherwise.
	if cmd.Flags().Changed("network.allow-public-traffic") {
		req.Network = &api.SandboxNetwork{AllowPublicTraffic: sandboxCreateNetworkAllowPublic}
	}

	sb, err := c.CreateSandbox(cmd.Context(), req)
	if err != nil {
		return err
	}

	// --detach: create only, print the sandbox ID and exit.
	if sandboxCreateDetach {
		fmt.Println(sb.SandboxID)
		return nil
	}

	// Default: open the interactive terminal.
	return helpers.RunTerminalSession(cmd.Context(), cmd.OutOrStdout(), os.Stdin, os.Stdout, sb, sandboxCreateCwd, sandboxCreateUser, envMap)
}

// sandboxListCmd's flag bound variables.
var (
	sandboxListMetadata     string
	sandboxListStates       []string
	sandboxListTemplate     string
	sandboxListStartedAfter string
	sandboxListOrder        string
	sandboxListLimit        int32
	sandboxListFormat       string
)

// sandboxListCmd implements `devbox sandbox list`, corresponding to GET /sandboxes.
var sandboxListCmd = &cobra.Command{
	Use:     "list|ls",
	Aliases: []string{"list", "ls"},
	Short:   i18n.T(i18n.Msg.Sandbox.ListShort),
	Args:    cobra.NoArgs,
	RunE:    runError(runSandboxList),
}

// runSandboxList validates the filters, queries the sandboxes and prints them
// as a table or JSON.
func runSandboxList(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	// Validate order/time before assembling params so nothing is set twice.
	if sandboxListOrder != "" && sandboxListOrder != "asc" && sandboxListOrder != "desc" {
		return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidOrder), sandboxListOrder)
	}
	if sandboxListStartedAfter != "" {
		if _, err := time.Parse(time.RFC3339, sandboxListStartedAfter); err != nil {
			return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidStartedAfter), sandboxListStartedAfter)
		}
	}
	params := &api.ListSandboxesParams{
		Metadata:     api.OptString(sandboxListMetadata),
		Template:     api.OptString(sandboxListTemplate),
		StartedAfter: api.OptString(sandboxListStartedAfter),
		Order:        api.OptString(sandboxListOrder),
	}
	if cmd.Flags().Changed("state") {
		states, err := helpers.ParseStates(sandboxListStates)
		if err != nil {
			return err
		}
		params.State = &states
	}
	// Aligned with e2b: 0 means no limit (the limit parameter is not sent).
	if sandboxListLimit < 0 || sandboxListLimit > 1000 {
		return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidLimit), sandboxListLimit)
	}
	if sandboxListLimit != 0 {
		params.Limit = api.PInt(sandboxListLimit)
	}

	res, err := c.ListSandboxes(cmd.Context(), params)
	if err != nil {
		return err
	}
	if len(res.Sandboxes) == 0 && out(cmd) == output.FormatText {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.ListEmpty))
		return nil
	}
	// Redact sensitive fields (connectToken/tokenExpiration) from JSON output,
	// consistent with `info -f json`.
	jsonSandboxes := make([]map[string]any, 0, len(res.Sandboxes))
	for i := range res.Sandboxes {
		jsonSandboxes = append(jsonSandboxes, helpers.SandboxJSON(&res.Sandboxes[i]))
	}
	rows := make([][]string, 0, len(res.Sandboxes))
	for _, sb := range res.Sandboxes {
		rows = append(rows, []string{
			sb.SandboxID,
			sb.TemplateID,
			output.FormatTime(sb.StartedAt),
			output.FormatTimePtr(sb.EndAt),
			string(sb.State),
			strconv.Itoa(int(sb.CpuCount)),
			strconv.Itoa(int(sb.MemoryMB)),
			strconv.Itoa(int(sb.DiskSizeMB)),
			output.FormatMetadata(sb.Metadata),
		})
	}
	return output.Print(cmd.OutOrStdout(), out(cmd), output.Spec{
		JSON:    jsonSandboxes,
		Headers: []string{"SANDBOX ID", "TEMPLATE ID", "STARTED AT", "END AT", "STATE", "VCPUS", "RAM(MB)", "DISK(MB)", "METADATA"},
		Rows:    rows,
	})
}

// sandboxInfoCmd implements `devbox sandbox info <sandboxID>`, corresponding to GET /sandboxes/{id}.
var sandboxInfoFormat string

var sandboxInfoCmd = &cobra.Command{
	Use:     "info|in <sandboxID>",
	Aliases: []string{"info", "in"},
	Short:   i18n.T(i18n.Msg.Sandbox.InfoShort),
	Args:    cobra.ExactArgs(1),
	RunE:    runError(runSandboxInfo),
}

// runSandboxInfo fetches one sandbox and prints its details as key-value
// pairs or JSON.
func runSandboxInfo(cmd *cobra.Command, args []string) error {
	if err := helpers.ValidateSandboxID(args[0]); err != nil {
		return err
	}
	c, err := newClient()
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
		{Key: "Started at", Value: output.FormatTime(sb.StartedAt)},
		{Key: "End at", Value: output.FormatTimePtr(sb.EndAt)},
		{Key: "vCPUs", Value: strconv.Itoa(int(sb.CpuCount))},
		{Key: "RAM(MB)", Value: strconv.Itoa(int(sb.MemoryMB))},
		{Key: "Disk(MB)", Value: strconv.Itoa(int(sb.DiskSizeMB))},
		{Key: "Lifecycle", Value: output.JSONString(sb.Lifecycle)},
		{Key: "Metadata", Value: output.FormatMetadata(sb.Metadata)},
		{Key: "Tunnel ID", Value: output.Dash(helpers.StrFromPtr(sb.TunnelId))},
		{Key: "Domain", Value: output.Dash(helpers.StrFromPtr(sb.Domain))},
	}
	return output.Print(cmd.OutOrStdout(), out(cmd), output.Spec{JSON: helpers.SandboxJSON(sb), KV: kv})
}

// sandboxKillCmd's flag bound variables.
var (
	sandboxKillAll      bool
	sandboxKillState    []string
	sandboxKillMetadata string
)

// sandboxKillCmd implements `devbox sandbox kill [sandboxID...]`, corresponding to DELETE /sandboxes/{id}.
// Aligned with e2b: multiple IDs are killed one by one, or --all kills in batch
// filtered by state/metadata (mutually exclusive with IDs).
var sandboxKillCmd = &cobra.Command{
	Use:     "kill|kl [sandboxID...]",
	Aliases: []string{"kill", "kl"},
	Short:   i18n.T(i18n.Msg.Sandbox.KillShort),
	Args:    cobra.ArbitraryArgs,
	RunE:    runError(runSandboxKill),
}

// runSandboxKill resolves the target sandbox IDs (explicit args or --all
// filtered by state/metadata) and deletes them one by one.
func runSandboxKill(cmd *cobra.Command, args []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}

	ids := args
	if sandboxKillAll {
		if len(ids) > 0 {
			return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.KillAllConflict))
		}
		// List by state/metadata then kill each; the state defaults to running.
		states := []api.SandboxState{api.Running}
		if cmd.Flags().Changed("state") {
			parsed, err := helpers.ParseStates(sandboxKillState)
			if err != nil {
				return err
			}
			states = parsed
		}
		params := &api.ListSandboxesParams{
			State:    &states,
			Metadata: api.OptString(sandboxKillMetadata),
		}
		for {
			res, listErr := c.ListSandboxes(cmd.Context(), params)
			if listErr != nil {
				return listErr
			}
			for _, sb := range res.Sandboxes {
				ids = append(ids, sb.SandboxID)
			}
			if res.NextToken == "" {
				break
			}
			params.NextToken = api.PString(res.NextToken)
		}
	} else if len(ids) == 0 {
		return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.KillRequiresArgs))
	}

	for _, id := range ids {
		if err := helpers.ValidateSandboxID(id); err != nil {
			return err
		}
	}
	failed := 0
	for _, id := range ids {
		if err := c.DeleteSandbox(cmd.Context(), id); err != nil {
			// Aligned with e2b: failures do not abort the remaining kills,
			// but the command exits non-zero when any deletion failed.
			failed++
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", id, err)
			continue
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.Killed)+"\n", id)
	}
	if failed > 0 {
		return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.KillPartialFailed), failed, len(ids))
	}
	return nil
}

// sandboxResumeCmd's flag bound variables.
var sandboxResumeTimeout int32

// sandboxPauseCmd implements `devbox sandbox pause <sandboxID>`, corresponding to POST /sandboxes/{id}/pause.
var sandboxPauseCmd = &cobra.Command{
	Use:     "pause|ps <sandboxID>",
	Aliases: []string{"pause", "ps"},
	Short:   i18n.T(i18n.Msg.Sandbox.PauseShort),
	Args:    cobra.ExactArgs(1),
	RunE:    runError(runSandboxPause),
}

// runSandboxPause pauses one sandbox.
func runSandboxPause(cmd *cobra.Command, args []string) error {
	if err := helpers.ValidateSandboxID(args[0]); err != nil {
		return err
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	if err := c.PauseSandbox(cmd.Context(), args[0]); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.Paused)+"\n", args[0])
	return nil
}

// sandboxResumeCmd implements `devbox sandbox resume <sandboxID>`, corresponding to POST /sandboxes/{id}/resume.
var sandboxResumeCmd = &cobra.Command{
	Use:     "resume|rs <sandboxID>",
	Aliases: []string{"resume", "rs"},
	Short:   i18n.T(i18n.Msg.Sandbox.ResumeShort),
	Args:    cobra.ExactArgs(1),
	RunE:    runError(runSandboxResume),
}

// runSandboxResume resumes one paused sandbox; an explicit -t overrides the TTL.
func runSandboxResume(cmd *cobra.Command, args []string) error {
	if err := helpers.ValidateSandboxID(args[0]); err != nil {
		return err
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	// With -t omitted the timeout field is not sent (the sandbox keeps its current TTL); an explicit value is validated and set.
	opt := api.TimeoutOption{}
	if cmd.Flags().Changed("timeout") {
		if err := helpers.ValidateTimeoutSeconds(sandboxResumeTimeout); err != nil {
			return err
		}
		opt.Timeout = api.PInt32(sandboxResumeTimeout)
	}
	_, err = c.ResumeSandbox(cmd.Context(), args[0], opt)
	if err != nil {
		return err
	}
	// Aligned with e2b: single-line output, no fields.
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.Resumed)+"\n", args[0])
	return nil
}

// sandboxConnectCmd's flag bound variables.
var (
	sandboxConnectTimeout int32
	sandboxConnectCwd     string
	sandboxConnectEnv     []string
	sandboxConnectUser    string
)

// sandboxConnectCmd implements `devbox sandbox connect <sandboxID>`, corresponding to POST /sandboxes/{id}/connect.
// After obtaining the connection credentials the interactive terminal opens (aligned with e2b's connect):
// the local terminal becomes the sandbox shell; type exit to leave.
var sandboxConnectCmd = &cobra.Command{
	Use:     "connect|cn <sandboxID>",
	Aliases: []string{"connect", "cn"},
	Short:   i18n.T(i18n.Msg.Sandbox.ConnectShort),
	Args:    cobra.ExactArgs(1),
	RunE:    runError(runSandboxConnect),
}

// runSandboxConnect obtains the connection credentials and opens the
// interactive terminal (aligned with e2b's connect): the local terminal
// becomes the sandbox shell; type exit to leave.
func runSandboxConnect(cmd *cobra.Command, args []string) error {
	if err := helpers.ValidateSandboxID(args[0]); err != nil {
		return err
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	// With -t omitted the timeout field is not sent (the sandbox keeps its current TTL); an explicit value is validated and set.
	opt := api.TimeoutOption{}
	if cmd.Flags().Changed("timeout") {
		if err := helpers.ValidateTimeoutSeconds(sandboxConnectTimeout); err != nil {
			return err
		}
		opt.Timeout = api.PInt32(sandboxConnectTimeout)
	}
	sb, err := c.ConnectSandbox(cmd.Context(), args[0], opt)
	if err != nil {
		return err
	}
	envMap, err := helpers.ParseKV(sandboxConnectEnv)
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidEnv), err)
	}
	return helpers.RunTerminalSession(cmd.Context(), cmd.OutOrStdout(), os.Stdin, os.Stdout, sb, sandboxConnectCwd, sandboxConnectUser, envMap)
}

// sandboxTimeoutCmd's flag bound variables.
var sandboxTimeoutValue int32

// sandboxTimeoutCmd implements `devbox sandbox timeout <sandboxID>`, corresponding to POST /sandboxes/{id}/timeout.
var sandboxTimeoutCmd = &cobra.Command{
	Use:     "timeout|to <sandboxID>",
	Aliases: []string{"timeout", "to"},
	Short:   i18n.T(i18n.Msg.Sandbox.TimeoutShort),
	Args:    cobra.ExactArgs(1),
	RunE:    runError(runSandboxTimeout),
}

// runSandboxTimeout sets the sandbox TTL; -t is optional: omitted uses the
// default 300, an explicit value is range-checked (10-86400).
func runSandboxTimeout(cmd *cobra.Command, args []string) error {
	if err := helpers.ValidateSandboxID(args[0]); err != nil {
		return err
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	if cmd.Flags().Changed("timeout") {
		if err := helpers.ValidateTimeoutSeconds(sandboxTimeoutValue); err != nil {
			return err
		}
	}
	if err := c.SetSandboxTimeout(cmd.Context(), args[0], api.TimeoutOption{
		Timeout: api.PInt32(sandboxTimeoutValue),
	}); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.TimeoutSet)+"\n", args[0], sandboxTimeoutValue)
	return nil
}

// sandboxRefreshCmd's flag bound variables.
var sandboxRefreshDuration int32

// sandboxRefreshCmd implements `devbox sandbox refresh <sandboxID>`, corresponding to POST /sandboxes/{id}/refreshes.
var sandboxRefreshCmd = &cobra.Command{
	Use:     "refresh|rf <sandboxID>",
	Aliases: []string{"refresh", "rf"},
	Short:   i18n.T(i18n.Msg.Sandbox.RefreshShort),
	Args:    cobra.ExactArgs(1),
	RunE:    runError(runSandboxRefresh),
}

// runSandboxRefresh extends the sandbox TTL; -d is optional: omitted uses the
// default 300, an explicit value is range-checked (10-86400).
func runSandboxRefresh(cmd *cobra.Command, args []string) error {
	if err := helpers.ValidateSandboxID(args[0]); err != nil {
		return err
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	if cmd.Flags().Changed("duration") {
		if err := helpers.ValidateTimeoutSeconds(sandboxRefreshDuration); err != nil {
			return err
		}
	}
	if err := c.RefreshSandbox(cmd.Context(), args[0], api.RefreshSandboxJSONRequestBody(api.RefreshSandboxJSONBody{
		Duration: sandboxRefreshDuration,
	})); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), i18n.T(i18n.Msg.Sandbox.Refreshed)+"\n", args[0], sandboxRefreshDuration)
	return nil
}

// sandboxExecCmd's flag bound variables.
var (
	sandboxExecBackground bool
	sandboxExecCwd        string
	sandboxExecEnv        []string
	sandboxExecUser       string
)

// sandboxExecCmd implements `devbox sbx exec <sandboxID> <command...>`.
// Aligned with e2b: everything after the sandbox ID is passed through to the remote command,
// so devbox options must come before the sandbox ID; the command runs via /bin/bash -l -c inside the sandbox
// and a non-zero exit code is returned as the local exit code.
var sandboxExecCmd = &cobra.Command{
	Use:     "exec|ex <sandboxID> <command...>",
	Aliases: []string{"exec", "ex"},
	Short:   i18n.T(i18n.Msg.Sandbox.ExecShort),
	Long:    i18n.T(i18n.Msg.Sandbox.ExecLong),
	Args:    cobra.MinimumNArgs(2),
	RunE:    runError(runSandboxExec),
}

// runSandboxExec runs a remote command inside the sandbox via
// /bin/bash -l -c and returns the remote exit code as the local exit code.
func runSandboxExec(cmd *cobra.Command, args []string) error {
	if err := helpers.ValidateSandboxID(args[0]); err != nil {
		return err
	}
	c, err := newClient()
	if err != nil {
		return err
	}
	cmdStr, err := helpers.BuildRemoteCommand(args[1:])
	if err != nil {
		return err
	}
	envMap, err := helpers.ParseKV(sandboxExecEnv)
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.Msg.Sandbox.InvalidEnv), err)
	}
	// Obtain the connection credentials (a paused sandbox is resumed automatically); no sandbox TTL is set.
	sb, err := c.ConnectSandbox(cmd.Context(), args[0], api.TimeoutOption{})
	if err != nil {
		return err
	}
	if sb.Domain == nil || *sb.Domain == "" {
		return helpers.MissingDomainError()
	}
	if sb.ConnectToken == nil || *sb.ConnectToken == "" {
		return fmt.Errorf("%s", i18n.T(i18n.Msg.Sandbox.MissingConnectToken))
	}
	pc, err := client.NewProcessClient(*sb.Domain, args[0], *sb.ConnectToken, sandboxExecUser, nil)
	if err != nil {
		return err
	}
	cfg := &process.ProcessConfig{
		Cmd:  "/bin/bash",
		Args: []string{"-l", "-c", cmdStr},
		Envs: envMap,
		Cwd:  api.OptString(sandboxExecCwd),
	}
	piped := helpers.PipedStdin()
	stream, err := pc.Start(cmd.Context(), cfg, piped)
	if err != nil {
		return err
	}
	if sandboxExecBackground {
		return helpers.ExecBackground(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), pc, stream, piped)
	}
	return helpers.ExecForeground(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), pc, stream, piped)
}

func init() {
	sandboxCreateCmd.Flags().Int32VarP(&sandboxCreateTimeout, "timeout", "t", 300,
		i18n.T(i18n.Msg.Sandbox.FlagTimeout))
	sandboxCreateCmd.Flags().StringArrayVarP(&sandboxCreateMetadata, "metadata", "m", nil,
		i18n.T(i18n.Msg.Sandbox.FlagMetadata))
	sandboxCreateCmd.Flags().StringArrayVarP(&sandboxCreateEnv, "env", "e", nil,
		i18n.T(i18n.Msg.Sandbox.FlagEnv))
	sandboxCreateCmd.Flags().StringVarP(&sandboxCreateBuildID, "build-id", "b", "",
		i18n.T(i18n.Msg.Sandbox.FlagBuildID))
	sandboxCreateCmd.Flags().StringVar(&sandboxCreateOnTimeout, "lifecycle.ontimeout", "",
		i18n.T(i18n.Msg.Sandbox.FlagOnTimeout))
	sandboxCreateCmd.Flags().BoolVar(&sandboxCreateAutoResume, "lifecycle.autoresume", false,
		i18n.T(i18n.Msg.Sandbox.FlagAutoResume))
	sandboxCreateCmd.Flags().BoolVar(&sandboxCreateNetworkAllowPublic, "network.allow-public-traffic", false,
		i18n.T(i18n.Msg.Sandbox.FlagNetworkPublic))
	sandboxCreateCmd.Flags().BoolVarP(&sandboxCreateDetach, "detach", "d", false,
		i18n.T(i18n.Msg.Sandbox.FlagDetach))
	sandboxCreateCmd.Flags().StringVarP(&sandboxCreateCwd, "cwd", "c", "",
		i18n.T(i18n.Msg.Sandbox.FlagCwd))
	sandboxCreateCmd.Flags().StringVarP(&sandboxCreateUser, "user", "u", "",
		i18n.T(i18n.Msg.Sandbox.FlagUser))

	sandboxListCmd.Flags().StringVarP(&sandboxListMetadata, "metadata", "m", "",
		i18n.T(i18n.Msg.Sandbox.FlagMetadataFilter))
	sandboxListCmd.Flags().StringVarP(&sandboxListTemplate, "template", "t", "",
		i18n.T(i18n.Msg.Sandbox.FlagTemplateFilter))
	sandboxListCmd.Flags().StringVar(&sandboxListStartedAfter, "started-after", "",
		i18n.T(i18n.Msg.Sandbox.FlagStartedAfter))
	sandboxListCmd.Flags().StringVarP(&sandboxListOrder, "order", "o", "",
		i18n.T(i18n.Msg.Sandbox.FlagOrder))
	sandboxListCmd.Flags().StringSliceVarP(&sandboxListStates, "state", "s", nil,
		i18n.T(i18n.Msg.Sandbox.FlagState))
	sandboxListCmd.Flags().Int32VarP(&sandboxListLimit, "limit", "l", 100,
		i18n.T(i18n.Msg.Sandbox.FlagLimit))
	sandboxListCmd.Flags().StringVarP(&sandboxListFormat, "format", "f", "pretty",
		i18n.T(i18n.Msg.Sandbox.FlagFormat))

	sandboxKillCmd.Flags().BoolVarP(&sandboxKillAll, "all", "a", false,
		i18n.T(i18n.Msg.Sandbox.FlagKillAll))
	sandboxKillCmd.Flags().StringSliceVarP(&sandboxKillState, "state", "s", nil,
		i18n.T(i18n.Msg.Sandbox.FlagKillState))
	sandboxKillCmd.Flags().StringVarP(&sandboxKillMetadata, "metadata", "m", "",
		i18n.T(i18n.Msg.Sandbox.FlagKillMetadata))

	sandboxResumeCmd.Flags().Int32VarP(&sandboxResumeTimeout, "timeout", "t", 300,
		i18n.T(i18n.Msg.Sandbox.FlagTimeoutKeep))

	sandboxConnectCmd.Flags().Int32VarP(&sandboxConnectTimeout, "timeout", "t", 300,
		i18n.T(i18n.Msg.Sandbox.FlagTimeoutKeep))
	sandboxConnectCmd.Flags().StringVarP(&sandboxConnectCwd, "cwd", "c", "",
		i18n.T(i18n.Msg.Sandbox.FlagCwd))
	sandboxConnectCmd.Flags().StringArrayVarP(&sandboxConnectEnv, "env", "e", nil,
		i18n.T(i18n.Msg.Sandbox.FlagEnvTerm))
	sandboxConnectCmd.Flags().StringVarP(&sandboxConnectUser, "user", "u", "",
		i18n.T(i18n.Msg.Sandbox.FlagUser))

	sandboxTimeoutCmd.Flags().Int32VarP(&sandboxTimeoutValue, "timeout", "t", 300,
		i18n.T(i18n.Msg.Sandbox.FlagNewTimeout))
	sandboxInfoCmd.Flags().StringVarP(&sandboxInfoFormat, "format", "f", "pretty",
		i18n.T(i18n.Msg.Sandbox.FlagFormat))

	sandboxRefreshCmd.Flags().Int32VarP(&sandboxRefreshDuration, "duration", "d", 300,
		i18n.T(i18n.Msg.Sandbox.FlagDuration))

	sandboxExecCmd.Flags().BoolVarP(&sandboxExecBackground, "background", "b", false,
		i18n.T(i18n.Msg.Sandbox.FlagBackground))
	sandboxExecCmd.Flags().StringVarP(&sandboxExecCwd, "cwd", "c", "",
		i18n.T(i18n.Msg.Sandbox.FlagCwd))
	sandboxExecCmd.Flags().StringArrayVarP(&sandboxExecEnv, "env", "e", nil,
		i18n.T(i18n.Msg.Sandbox.FlagEnv))
	sandboxExecCmd.Flags().StringVarP(&sandboxExecUser, "user", "u", "",
		i18n.T(i18n.Msg.Sandbox.FlagUser))
	// Everything after the sandbox ID is passed through to the remote command.
	sandboxExecCmd.Flags().SetInterspersed(false)

	sandboxCmd.AddCommand(
		sandboxCreateCmd,
		sandboxListCmd,
		sandboxInfoCmd,
		sandboxKillCmd,
		sandboxPauseCmd,
		sandboxResumeCmd,
		sandboxConnectCmd,
		sandboxExecCmd,
		sandboxTimeoutCmd,
		sandboxRefreshCmd,
	)
}
