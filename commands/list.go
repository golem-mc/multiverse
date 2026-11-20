package commands

import (
	"slices"

	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, listCommands) }

// listCommands is /mv list. MV: ListCommand
func listCommands(c *core.Core) []*core.Cmd {
	flags := append(core.PageFilterFlags(), &core.Flag{Name: "--raw", Aliases: []string{"-r"}})
	return []*core.Cmd{{
		Name: "list", Legacy: []string{"mvlist", "mvl"},
		Permission: "multiverse.core.list.worlds", Syntax: "[--filter <filter>] [--page <page>] [--raw]",
		Description: "mv-core.list.description", MaxArgs: 0, Flags: flags,
		Run: func(ctx *core.Ctx) error {
			d, err := ctx.PagedDisplay()
			if err != nil {
				return err
			}
			h := core.M("mv-core.list.header")
			nc := core.M("mv-core.list.nocontent")
			d.Header, d.NoContent = &h, &nc
			d.Send(ctx.Issuer, listContents(ctx, ctx.Has("--raw")))
			return nil
		},
	}}
}

// listContents answers the worlds the sender can access: loaded ones with
// their environment, then unloaded ones, each group sorted.
func listContents(ctx *core.Ctx, raw bool) []string {
	name := func(w *core.World) string {
		if raw {
			return w.Name()
		}
		return w.AliasOrName()
	}
	var loaded, unloaded []string
	for _, w := range ctx.Core.Worlds.Loaded() {
		if !ctx.Core.CanAccess(ctx.Sender, w) || w.Hidden() && !ctx.Has("multiverse.core.modify") && !ctx.Issuer.Has("multiverse.core.modify") {
			continue
		}
		hidden := ""
		if w.Hidden() {
			hidden = "§7[H] "
		}
		loaded = append(loaded, hidden+name(w)+" - "+envColour(w.Environment()))
	}
	for _, w := range ctx.Core.Worlds.Unloaded() {
		if !ctx.Core.CanAccess(ctx.Sender, w) {
			continue
		}
		unloaded = append(unloaded, "§7"+name(w)+" - UNLOADED")
	}
	slices.Sort(loaded)
	slices.Sort(unloaded)
	return append(loaded, unloaded...)
}

// envColour writes an environment as Bukkit's enum name, coloured.
func envColour(env string) string {
	colour := map[string]string{"nether": "§c", "normal": "§a", "the_end": "§b"}[env]
	if colour == "" {
		colour = "§6"
	}
	return colour + upper(env)
}
