package commands

import (
	"strconv"
	"strings"

	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, rootCommands) }

// rootCommands are /mv confirm, help and version. MV: ConfirmCommand,
// HelpCommand, VersionCommand
func rootCommands(c *core.Core) []*core.Cmd {
	return []*core.Cmd{{
		Name: "confirm", Legacy: []string{"mvconfirm"},
		Permission: "multiverse.core.confirm", Syntax: "[otp]",
		Description: "mv-core.confirm.description", MaxArgs: 1,
		Run: func(ctx *core.Ctx) error {
			otp := ctx.Arg(0)
			if otp == "" {
				otp = "0"
			}
			return ctx.Core.Commands.RunConfirmed(ctx.Issuer, otp)
		},
	}, {
		Name: "help", Legacy: []string{"mvhelp"},
		Permission: "multiverse.core.help", Syntax: "[filter] [page]",
		Description: "mv-core.usage.description", MaxArgs: 2,
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos > 0 {
				return nil
			}
			var out []string
			for _, cmd := range ctx.Core.Commands.All() {
				out = append(out, strings.Fields(cmd.Name)[0])
			}
			return out
		},
		Run: func(ctx *core.Ctx) error {
			filter, page := ctx.Arg(0), 1
			if n, err := strconv.Atoi(filter); err == nil {
				filter, page = "", n
			} else if n, err := strconv.Atoi(ctx.Arg(1)); err == nil {
				page = n
			}
			ctx.Core.Commands.Help(ctx.Issuer, "mv", ctx.Core.Commands.All(), filter, page)
			return nil
		},
	}, {
		Name: "version", Legacy: []string{"mvversion"},
		Permission: "multiverse.core.version", Description: "mv-core.version.description",
		Run: func(ctx *core.Ctx) error {
			ctx.Info(core.M("mv-core.version.mv", "{version}", core.Version))
			ctx.Info(core.M("mv-core.version.authors", "{authors}", strings.Join(core.Authors, ", ")))
			ctx.Info(core.M("mv-core.version.secretcode"))
			return nil
		},
	}}
}
