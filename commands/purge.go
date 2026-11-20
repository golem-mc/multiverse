package commands

import (
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, purgeCommands) }

// purgeCommands are /mv purge-entities and /mv purge-all-entities. MV:
// PurgeEntitiesCommand, PurgeAllEntitiesCommand
func purgeCommands(c *core.Core) []*core.Cmd {
	return []*core.Cmd{{
		Name: "purge-entities", Permission: "multiverse.core.purge", Syntax: "[world]", MaxArgs: 1,
		Complete: func(ctx *core.Ctx, _ int) []string { return ctx.Core.WorldNames("loaded") },
		Run: func(ctx *core.Ctx) error {
			w, err := optionalWorld(ctx, 0, true)
			if err != nil {
				return err
			}
			n := w.PurgeDisallowedEntities()
			ctx.Send(core.M("mv-core.purgeentities.success", "{count}", n, "{world}", w.Name()))
			return nil
		},
	}, {
		Name: "purge-all-entities", Permission: "multiverse.core.purgeall", Syntax: "[world] [spawn-categories]",
		MaxArgs: 2,
		Complete: func(ctx *core.Ctx, pos int) []string {
			if pos == 0 {
				return append(ctx.Core.WorldNames("loaded"), core.SpawnCategories...)
			}
			return core.SpawnCategories
		},
		Run: func(ctx *core.Ctx) error {
			w, consumed, err := ctx.WorldArg(0, true)
			if err != nil {
				return err
			}
			arg := ctx.Arg(0)
			if consumed {
				arg = ctx.Arg(1)
			}
			if arg == "" {
				n := w.PurgeEntities(func(plugin.Entity) bool { return true })
				ctx.Send(core.M("mv-core.purgeallentities.success", "{count}", n, "{world}", w.Name()))
				return nil
			}
			categories := spawnCategoriesArg(arg)
			if len(categories) == 0 {
				// Multiverse ignores the names it does not know and purges every
				// entity when none is left; a typo should not do that.
				_, err := pickOne(arg, core.SpawnCategories)
				return err
			}
			n := w.PurgeEntities(func(e plugin.Entity) bool {
				return slices.Contains(categories, core.SpawnCategoryOf(e.Type().Name()))
			})
			names := make([]string, len(categories))
			for i, cat := range categories {
				names[i] = strings.ToUpper(cat)
			}
			ctx.Send(core.M("mv-core.purgeallentities.success.categories", "{count}", n, "{world}", w.Name(),
				"{categories}", strings.Join(names, ", ")))
			return nil
		},
	}}
}

// spawnCategoriesArg reads comma-separated spawn categories, skipping the
// names it does not know. MV: MVCommandContexts.parseSpawnCategories
func spawnCategoriesArg(s string) []string {
	var out []string
	for name := range strings.SplitSeq(s, ",") {
		if cat, err := pickOne(name, core.SpawnCategories); err == nil && !slices.Contains(out, cat) {
			out = append(out, cat)
		}
	}
	return out
}
