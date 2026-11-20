package commands

import (
	"maps"
	"slices"
	"strings"

	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, metaCommands) }

// metaCommands are /mv meta info and /mv meta modify. MV: MetaCommand
func metaCommands(c *core.Core) []*core.Cmd {
	return []*core.Cmd{{
		Name: "meta info", Permission: "multiverse.core.meta.info",
		Syntax: "[world] [--page <page>] [--filter <filter>]", Description: "mv-core.meta.info.description",
		MaxArgs: 1, Flags: core.PageFilterFlags(),
		Complete: func(ctx *core.Ctx, _ int) []string { return ctx.Core.WorldNames("both") },
		Run: func(ctx *core.Ctx) error {
			w, err := optionalWorld(ctx, 0, false)
			if err != nil {
				return err
			}
			d, err := ctx.PagedDisplay()
			if err != nil {
				return err
			}
			h := core.M("mv-core.meta.info.header", "{world}", w.Name())
			nc := core.M("mv-core.meta.info.nocontent", "{world}", w.Name())
			d.Header, d.NoContent = &h, &nc
			meta := w.Meta()
			keys := slices.Sorted(maps.Keys(meta))
			vals := map[string]*string{}
			for _, k := range keys {
				v := meta[k]
				vals[k] = &v
			}
			d.Send(ctx.Issuer, core.MapLines(ctx.Issuer, keys, vals, "§b", "§f"))
			return nil
		},
	}, {
		Name: "meta modify", Permission: "multiverse.core.meta.modify",
		Syntax: "[world] <set|remove> <key> [value]", Description: "mv-core.meta.modify.description",
		MinArgs: 2, MaxArgs: 4,
		Complete: func(ctx *core.Ctx, pos int) []string {
			w, consumed, _ := ctx.WorldArg(0, false)
			if consumed {
				pos--
			}
			switch pos {
			case 0:
				if consumed {
					return []string{"set", "remove"}
				}
				return append(ctx.Core.WorldNames("both"), "set", "remove")
			case 1:
				if w != nil {
					return slices.Sorted(maps.Keys(w.Meta()))
				}
			}
			return nil
		},
		Run: func(ctx *core.Ctx) error {
			w, consumed, err := ctx.WorldArg(0, false)
			if err != nil {
				return err
			}
			args := ctx.Args
			if consumed {
				args = args[1:]
			}
			if len(args) < 2 {
				return core.ErrUsage
			}
			action, key := args[0], args[1]
			value := "null"
			if len(args) > 2 {
				value = args[2]
			}
			switch strings.ToLower(action) {
			case "set":
				// Multiverse stores the missing value as null, which saves as nothing.
				stored := value
				if len(args) < 3 {
					stored = ""
				}
				w.Meta()[key] = stored
				ctx.Info(core.M("mv-core.meta.modify.set.success", "{key}", key, "{value}", value, "{world}", w.Name()))
			case "remove":
				delete(w.Meta(), key)
				ctx.Info(core.M("mv-core.meta.modify.remove.success", "{key}", key, "{world}", w.Name()))
			default:
				ctx.Error(core.M("mv-core.meta.modify.invalidaction", "{action}", action))
			}
			if err := ctx.Core.Worlds.Save(); err != nil {
				ctx.Core.Log.Error("Failed to save worlds.yml", "error", err)
			}
			return nil
		},
	}}
}
