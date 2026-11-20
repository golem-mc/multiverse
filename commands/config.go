package commands

import (
	"strconv"

	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, configCommands) }

// configCommands are /mv config, /mv debug and /mv reload. MV:
// ConfigCommand, DebugCommand, ReloadCommand
func configCommands(c *core.Core) []*core.Cmd {
	return []*core.Cmd{{
		Name: "config", Legacy: []string{"mvconfig", "mvconf"},
		Permission: "multiverse.core.config", Syntax: "<name> [value]",
		Description: "mv-core.config.description", MinArgs: 1, MaxArgs: 2,
		Complete: func(ctx *core.Ctx, pos int) []string {
			nodes := ctx.Core.Config.Nodes()
			if pos == 0 {
				return nodes.Names()
			}
			if n, ok := nodes.Find(ctx.Arg(0)); ok {
				return core.SuggestValues(n, "")
			}
			return nil
		},
		Run: func(ctx *core.Ctx) error {
			name := ctx.Arg(0)
			if len(ctx.Args) < 2 {
				n, ok := ctx.Core.Config.Nodes().Find(name)
				if !ok {
					ctx.Send(core.M("mv-core.config.show.error", "{name}", name,
						"{error}", core.M("mv-core.config.node.notfound", "{node}", name)))
					return nil
				}
				ctx.Send(core.M("mv-core.config.show.success", "{name}", name,
					"{value}", core.FormatValue(ctx.Core.Config.Get(n))))
				return nil
			}
			value := ctx.Arg(1)
			if err := setConfig(ctx.Core, name, value); err != nil {
				ctx.Send(core.M("mv-core.config.set.error", "{name}", name, "{value}", value,
					"{error}", core.ErrorMessage(err)))
				return nil
			}
			n, _ := ctx.Core.Config.Nodes().Find(name)
			ctx.Send(core.M("mv-core.config.set.success", "{name}", name,
				"{value}", core.FormatValue(ctx.Core.Config.Get(n))))
			return nil
		},
	}, {
		Name: "debug", Legacy: []string{"mvdebug"},
		Permission: "multiverse.core.debug", Syntax: "<level>",
		Description: "mv-core.debug.info.description", MaxArgs: 1,
		Complete: func(*core.Ctx, int) []string { return []string{"0", "1", "2", "3"} },
		Run: func(ctx *core.Ctx) error {
			if len(ctx.Args) == 1 {
				level, err := strconv.Atoi(ctx.Arg(0))
				if err != nil {
					return core.InvalidArg(core.Lit("Please specify a number."))
				}
				if err := setConfig(ctx.Core, "global-debug", strconv.Itoa(level)); err != nil {
					return err
				}
			}
			if level := ctx.Core.Config.Int("misc.global-debug"); level == 0 {
				ctx.Info(core.M("mv-core.debug.info.off"))
			} else {
				ctx.Info(core.M("mv-core.debug.info.on", "{level}", level))
			}
			return nil
		},
	}, {
		Name: "reload", Legacy: []string{"mvreload", "mvr"},
		Permission: "multiverse.core.reload", Description: "mv-core.reload.description",
		Run: func(ctx *core.Ctx) error {
			ctx.Info(core.M("mv-core.reload.reloading"))
			files, err := ctx.Core.Reload()
			if err != nil {
				ctx.Core.Log.Error("reload failed", "error", err)
			}
			for _, f := range files {
				ctx.Send(core.Lit(f))
			}
			ctx.Info(core.M("mv-core.reload.success"))
			return nil
		},
	}}
}

// setConfig parses and sets a config.yml option by name and saves the file.
func setConfig(c *core.Core, name, value string) error {
	n, ok := c.Config.Nodes().Find(name)
	if !ok {
		return core.MsgError(core.M("mv-core.config.node.notfound", "{node}", name))
	}
	v, err := core.ParseString(n, value)
	if err != nil {
		return err
	}
	if err := c.Config.Set(n, v); err != nil {
		return err
	}
	return c.Config.Save()
}
