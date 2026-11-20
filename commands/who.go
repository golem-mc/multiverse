package commands

import (
	"slices"
	"strings"

	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, whoCommands) }

// whoCommands are /mv whoall and /mv who. MV: WhoCommand
func whoCommands(c *core.Core) []*core.Cmd {
	return []*core.Cmd{{
		Name: "whoall", Legacy: []string{"mvwhoall"}, Permission: "multiverse.core.list.who.all",
		Syntax: "[--page <page>] [--filter <filter>]", Description: "mv-core.who.all.description",
		MaxArgs: 0, Flags: core.PageFilterFlags(),
		Run: func(ctx *core.Ctx) error { return sendWho(ctx, ctx.Core.Worlds.Loaded(), true) },
	}, {
		Name: "who", Legacy: []string{"mvwho", "mvw"}, Permission: "multiverse.core.list.who",
		Syntax: "[world] [--page <page>] [--filter <filter>]", Description: "mv-core.who.description",
		MaxArgs: 1, Flags: core.PageFilterFlags(),
		Complete: func(ctx *core.Ctx, _ int) []string { return ctx.Core.WorldNames("both") },
		Run: func(ctx *core.Ctx) error {
			w, err := optionalWorld(ctx, 0, true)
			if err != nil {
				return err
			}
			return sendWho(ctx, []*core.World{w}, false)
		},
	}}
}

// sendWho lists the players of each world, sorted by world; skipEmpty
// leaves out the worlds nobody is in. MV: WhoCommand.getListDisplay
func sendWho(ctx *core.Ctx, worlds []*core.World, skipEmpty bool) error {
	d, err := ctx.PagedDisplay()
	if err != nil {
		return err
	}
	h := core.M("mv-core.who.header")
	d.Header = &h
	var keys []string
	vals := map[string]*string{}
	for _, w := range worlds {
		var names []string
		for _, p := range w.Players() {
			names = append(names, p.Name())
		}
		if len(names) == 0 && skipEmpty {
			continue
		}
		v := strings.Join(names, ", ")
		if len(names) == 0 {
			v = ctx.Format(core.M("mv-core.who.empty"))
		}
		keys = append(keys, w.AliasOrName())
		vals[w.AliasOrName()] = &v
	}
	slices.Sort(keys)
	d.Send(ctx.Issuer, core.MapLines(ctx.Issuer, keys, vals, "§b", "§f"))
	return nil
}
