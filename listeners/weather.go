package listeners

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, weather) }

// weather keeps rain and thunder from starting in worlds with allow-weather
// off. MV: MVWeatherListener
func weather(c *core.Core) {
	denied := func(h plugin.World) bool {
		w, ok := c.Worlds.Of(h)
		return ok && !w.Props.Bool(core.PropAllowWeather)
	}
	opts := plugin.ListenOptions{Priority: plugin.Normal, IgnoreCancelled: true}
	plugin.Listen(c.Server, func(e *plugin.WeatherChangeEvent) {
		if e.Raining && denied(e.World) {
			e.SetCancelled(true)
		}
	}, opts)
	plugin.Listen(c.Server, func(e *plugin.ThunderChangeEvent) {
		if e.Thundering && denied(e.World) {
			e.SetCancelled(true)
		}
	}, opts)
}
