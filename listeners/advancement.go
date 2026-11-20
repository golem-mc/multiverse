package listeners

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, advancements) }

// advancements stops criteria being granted in worlds with
// allow-advancement-grant off, but for the recipes vanilla unlocks right
// away. MV: MVAdvancementListener
func advancements(c *core.Core) {
	plugin.Listen(c.Server, func(e *plugin.PlayerAdvancementCriterionGrantEvent) {
		w, ok := c.Worlds.Of(e.Player.World())
		if ok && !w.Props.Bool(core.PropAdvancementGrant) && e.Criterion != "unlock_right_away" {
			c.Debug(3, "Advancement criterion cancelled: %s", e.Criterion)
			e.SetCancelled(true)
		}
	})
}
