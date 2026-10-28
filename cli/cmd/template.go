package cmd

import (
	"strconv"

	"github.com/huaweicloud/devspace-devbox/cli/internal/output"

	"github.com/spf13/cobra"
)

// templateCmd 实现 `devbox template`。
var templateCmd = &cobra.Command{
	// Use 里的 "template|tpl" 仅用于帮助展示；命令匹配靠 Aliases。
	Use:     "template|tpl",
	Aliases: []string{"template", "tpl"},
	Short:   "Manage sandbox templates",
}

// templateListCmd 实现 `devbox template list`，对应 GET /templates。
var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List templates",
	Args:  cobra.NoArgs,
	RunE: runError(func(cmd *cobra.Command, args []string) error {
		c, err := newClient(cmd)
		if err != nil {
			return err
		}
		tpls, err := c.ListTemplates(cmd.Context())
		if err != nil {
			return err
		}
		if len(tpls) == 0 && out(cmd) == output.FormatText {
			_, _ = cmd.OutOrStdout().Write([]byte("No templates found\n"))
			return nil
		}
		rows := make([][]string, 0, len(tpls))
		for _, t := range tpls {
			rows = append(rows, []string{
				t.TemplateID,
				t.Name,
				joinList(t.Aliases),
				t.BuildID,
				strconv.Itoa(int(t.CpuCount)),
				strconv.Itoa(int(t.MemoryMB)),
				strconv.Itoa(int(t.DiskSizeMB)),
			})
		}
		return output.Print(cmd.OutOrStdout(), out(cmd), output.Spec{
			JSON:    tpls,
			Headers: []string{"Template ID", "Name", "Aliases", "Build ID", "VCPU", "RAM(MB)", "Disk(MB)"},
			Rows:    rows,
		})
	}),
}

func init() {
	templateCmd.AddCommand(templateListCmd)
}
