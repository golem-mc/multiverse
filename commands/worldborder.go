package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, worldBorderCommands) }

const (
	tick    = time.Second / 20
	gameDay = 24000 * tick
	// maxBorderSize and maxBorderCenter are the bounds Paper's WorldBorder
	// checks; golem's API takes any value.
	maxBorderSize   = 5.9999968e7
	maxBorderCenter = 2.9999984e7
)

// worldBorderCommands are /mv worldborder's subcommands. golem keeps the
// warning time in ticks, as Paper does, so no time is rounded. MV:
// WorldBorderCommand
func worldBorderCommands(c *core.Core) []*core.Cmd {
	worlds := func(ctx *core.Ctx, _ int) []string { return ctx.Core.WorldNames("loaded") }
	// sub builds a subcommand whose world is the argument after its nargs
	// own ones.
	sub := func(name, syntax string, nargs, optional int, run func(ctx *core.Ctx, border borderFunc) error) *core.Cmd {
		return &core.Cmd{
			Name: "worldborder " + name, Permission: "multiverse.core.worldborder", Syntax: syntax,
			MinArgs: nargs, MaxArgs: nargs + optional + 1,
			Complete: func(ctx *core.Ctx, pos int) []string {
				if pos >= nargs+optional {
					return worlds(ctx, pos)
				}
				return nil
			},
			Run: func(ctx *core.Ctx) error {
				return run(ctx, func() (*core.World, plugin.WorldBorder, error) {
					w, err := optionalWorld(ctx, nargs+optional, true)
					if err != nil {
						return nil, nil, err
					}
					h, _ := w.Handle()
					return w, h.Border(), nil
				})
			},
		}
	}
	return []*core.Cmd{
		sub("add", "<size> [time] [world]", 1, 1, func(ctx *core.Ctx, border borderFunc) error {
			size, err := doubleArg(ctx.Arg(0))
			if err != nil {
				return err
			}
			d, err := tickDurationArg(ctx.Arg(1))
			if err != nil {
				return err
			}
			w, b, err := border()
			if err != nil {
				return err
			}
			return setBorder(ctx, w, b, b.Size()+size, d)
		}),
		sub("center", "<x> <z> [world]", 2, 0, func(ctx *core.Ctx, border borderFunc) error {
			x, err := doubleArg(ctx.Arg(0))
			if err != nil {
				return err
			}
			z, err := doubleArg(ctx.Arg(1))
			if err != nil {
				return err
			}
			w, b, err := border()
			if err != nil {
				return err
			}
			if cx, cz := b.Center(); cx == x && cz == z {
				ctx.Send(core.M("mv-core.worldborder.center.nothingchanged", "{world}", w.AliasOrName()))
				return nil
			}
			if max(x, -x, z, -z) > maxBorderCenter {
				return borderError(fmt.Sprintf("x and z must be between -%s and %s", core.JavaDouble(maxBorderCenter), core.JavaDouble(maxBorderCenter)))
			}
			b.SetCenter(x, z)
			cx, cz := b.Center()
			ctx.Send(core.M("mv-core.worldborder.center.success", "{x}", core.JavaDouble(cx), "{z}", core.JavaDouble(cz),
				"{world}", w.AliasOrName()))
			return nil
		}),
		sub("damage amount", "<damage> [world]", 1, 0, func(ctx *core.Ctx, border borderFunc) error {
			damage, err := doubleArg(ctx.Arg(0))
			if err != nil {
				return err
			}
			w, b, err := border()
			if err != nil {
				return err
			}
			if b.DamagePerBlock() == damage {
				ctx.Send(core.M("mv-core.worldborder.damageamount.nothingchanged", "{world}", w.AliasOrName()))
				return nil
			}
			b.SetDamagePerBlock(damage)
			ctx.Send(core.M("mv-core.worldborder.damageamount.success", "{amount}", core.JavaDouble(b.DamagePerBlock()),
				"{world}", w.AliasOrName()))
			return nil
		}),
		sub("damage buffer", "<distance> [world]", 1, 0, func(ctx *core.Ctx, border borderFunc) error {
			distance, err := doubleArg(ctx.Arg(0))
			if err != nil {
				return err
			}
			w, b, err := border()
			if err != nil {
				return err
			}
			if b.DamageBuffer() == distance {
				ctx.Send(core.M("mv-core.worldborder.damagebuffer.nothingchanged", "{world}", w.AliasOrName()))
				return nil
			}
			b.SetDamageBuffer(distance)
			ctx.Send(core.M("mv-core.worldborder.damagebuffer.success", "{distance}", core.JavaDouble(b.DamageBuffer()),
				"{world}", w.AliasOrName()))
			return nil
		}),
		sub("get", "[world]", 0, 0, func(ctx *core.Ctx, border borderFunc) error {
			w, b, err := border()
			if err != nil {
				return err
			}
			ctx.Send(core.M("mv-core.worldborder.get.size", "{size}", core.JavaDouble(b.Size()), "{world}", w.AliasOrName()))
			return nil
		}),
		sub("set", "<size> [time] [world]", 1, 1, func(ctx *core.Ctx, border borderFunc) error {
			size, err := doubleArg(ctx.Arg(0))
			if err != nil {
				return err
			}
			d, err := tickDurationArg(ctx.Arg(1))
			if err != nil {
				return err
			}
			w, b, err := border()
			if err != nil {
				return err
			}
			return setBorder(ctx, w, b, size, d)
		}),
		sub("warning distance", "<distance> [world]", 1, 0, func(ctx *core.Ctx, border borderFunc) error {
			distance, err := strconv.Atoi(ctx.Arg(0))
			if err != nil {
				return core.InvalidArg(core.Lit("Please specify a number."))
			}
			w, b, err := border()
			if err != nil {
				return err
			}
			if b.WarningDistance() == distance {
				ctx.Send(core.M("mv-core.worldborder.warningdistance.nothingchanged", "{world}", w.AliasOrName()))
				return nil
			}
			b.SetWarningDistance(distance)
			ctx.Send(core.M("mv-core.worldborder.warningdistance.success", "{distance}", b.WarningDistance(),
				"{world}", w.AliasOrName()))
			return nil
		}),
		sub("warning time", "<time> [world]", 1, 0, func(ctx *core.Ctx, border borderFunc) error {
			d, err := tickDurationArg(ctx.Arg(0))
			if err != nil {
				return err
			}
			w, b, err := border()
			if err != nil {
				return err
			}
			if b.WarningTime()/tick == d/tick {
				ctx.Send(core.M("mv-core.worldborder.warningtime.nothingchanged", "{world}", w.AliasOrName()))
				return nil
			}
			b.SetWarningTime(d)
			ctx.Send(core.M("mv-core.worldborder.warningtime.success", "{time}", seconds(b.WarningTime()),
				"{world}", w.AliasOrName()))
			return nil
		}),
	}
}

// borderFunc resolves a worldborder command's world, after its own
// arguments, as ACF resolves the parameters in order.
type borderFunc func() (*core.World, plugin.WorldBorder, error)

// setBorder moves the border to size over d. MV: WorldBorderCommand.onWorldBorderSet
func setBorder(ctx *core.Ctx, w *core.World, b plugin.WorldBorder, size float64, d time.Duration) error {
	previous := b.Size()
	if previous == size {
		ctx.Send(core.M("mv-core.worldborder.set.nothingchanged", "{world}", w.AliasOrName()))
		return nil
	}
	if size < 1 || size > maxBorderSize {
		return borderError("newSize must be between 1.0 and " + core.JavaDouble(maxBorderSize))
	}
	b.SetSize(size, d)
	if d < tick {
		ctx.Send(core.M("mv-core.worldborder.set.immediate", "{size}", core.JavaDouble(b.Size()), "{world}", w.AliasOrName()))
		return nil
	}
	// Multiverse swaps the two messages; they say what the border does here.
	key := "mv-core.worldborder.set.growing"
	if size < previous {
		key = "mv-core.worldborder.set.shrinking"
	}
	ctx.Send(core.M(key, "{size}", core.JavaDouble(size), "{time}", seconds(d), "{world}", w.AliasOrName()))
	return nil
}

// borderError is a failure Bukkit's WorldBorder would throw, reported as
// Multiverse reports one.
func borderError(msg string) error {
	return core.MsgError(core.M("mv-core.generic.error.details", "{error}", "java.lang.IllegalArgumentException: "+msg))
}

// doubleArg reads a number argument. ACF: double context
func doubleArg(s string) (float64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, core.InvalidArg(core.Lit("Please specify a number."))
	}
	return f, nil
}

// tickDurationArg reads a duration: ticks, or seconds with "s", game days
// with "d"; "" is none. MV: TickDuration.parseString
func tickDurationArg(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	unit, num := tick, s
	if n, ok := strings.CutSuffix(s, "s"); ok {
		unit, num = time.Second, n
	} else if n, ok := strings.CutSuffix(s, "d"); ok {
		unit, num = gameDay, n
	}
	n, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		return 0, core.InvalidArg(core.Lit("Invalid time duration format: \"" + s + "\". Use a number followed by an " +
			"optional suffix (s for seconds, d for game days) or just a number for ticks. Examples: 100, 5s, 2d"))
	}
	return time.Duration(n) * unit, nil
}

// seconds writes d as Multiverse's "%.2f" seconds.
func seconds(d time.Duration) string { return fmt.Sprintf("%.2f", d.Seconds()) }
