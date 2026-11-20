package core

import (
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// MVTeleportDestinationEvent fires before Multiverse sends an entity to a
// destination; cancelling stops it.
type MVTeleportDestinationEvent struct {
	plugin.Cancellable
	Destination Destination
	Teleportee  plugin.Entity
	// Teleporter is who asked, nil when the teleportee did.
	Teleporter plugin.CommandSender
}

// TeleportFailure is why a teleport did not happen; its message key is
// mv-core.teleportfailurereason.<reason>. MV: TeleportFailureReason
type TeleportFailure string

const (
	TeleportNullDestination TeleportFailure = "null.destination"
	TeleportNullLocation    TeleportFailure = "null.location"
	TeleportNullWorld       TeleportFailure = "null.world"
	TeleportUnsafe          TeleportFailure = "unsafe.location"
	TeleportFailed          TeleportFailure = "teleport.failed"
	TeleportCancelled       TeleportFailure = "event.cancelled"
)

// Message answers the failure's message.
func (f TeleportFailure) Message() Message { return M("mv-core.teleportfailurereason." + string(f)) }

// PassengerMode says what happens to riders and vehicles on a teleport.
// MV: PassengerModes
type PassengerMode struct{ DismountPassengers, PassengersFollow, DismountVehicle, VehicleFollow bool }

// PassengerModes are the passenger-mode values. MV: PassengerModes
var PassengerModes = map[string]PassengerMode{
	"default":             {},
	"dismount_passengers": {DismountPassengers: true},
	"dismount_vehicle":    {DismountPassengers: true, DismountVehicle: true},
	"dismount_all":        {DismountPassengers: true, DismountVehicle: true},
	"retain_passengers":   {DismountPassengers: true, PassengersFollow: true},
	"retain_vehicle":      {DismountVehicle: true, VehicleFollow: true},
	"retain_all":          {DismountPassengers: true, PassengersFollow: true, DismountVehicle: true, VehicleFollow: true},
}

// Teleport is one safety teleport being set up. MV: AsyncSafetyTeleporterAction
type Teleport struct {
	core *Core
	dest Destination
	loc  *plugin.Location
	// Safety looks for a safe spot near the target.
	Safety bool
	// By is who asked for the teleport, for the teleport intercept's entry
	// checks; nil means the teleportee.
	By        plugin.CommandSender
	Passenger PassengerMode
}

// TeleportTo starts a teleport to a destination, with the destination's
// safety and the configured passenger mode.
func (c *Core) TeleportTo(dest Destination) *Teleport {
	return &Teleport{core: c, dest: dest, Safety: dest != nil && dest.CheckSafety(), Passenger: c.passengerMode()}
}

// TeleportToLocation starts a safe teleport to a location.
func (c *Core) TeleportToLocation(loc plugin.Location) *Teleport {
	return &Teleport{core: c, loc: &loc, Safety: true, Passenger: c.passengerMode()}
}

func (c *Core) passengerMode() PassengerMode {
	return PassengerModes[strings.ToLower(c.Config.String("teleport.passenger-mode"))]
}

// Run teleports e, calling done on the tick goroutine with "" on success.
// The target's chunks load first, for the safety search.
// MV: AsyncSafetyTeleporterAction.teleportSingle
func (t *Teleport) Run(e plugin.Entity, done func(TeleportFailure)) {
	loc, fail := t.target(e)
	if fail != "" {
		done(fail)
		return
	}
	r := 0
	if t.Safety {
		r = t.core.Config.Int("teleport.safe-location-horizontal-search-radius")
	}
	WithChunks(loc.World, loc.Pos, r, func() {
		if t.Safety {
			safe, ok := FindSafe(loc, r, t.core.Config.Int("teleport.safe-location-vertical-search-radius"))
			if !ok {
				done(TeleportUnsafe)
				return
			}
			loc = safe
		}
		done(t.move(e, loc))
	})
}

// RunAll teleports each entity and reports once all are done: how many
// succeeded and how many failed for each reason.
func (t *Teleport) RunAll(es []plugin.Entity, done func(ok int, failed map[TeleportFailure]int)) {
	left, ok, failed := len(es), 0, map[TeleportFailure]int{}
	if left == 0 {
		done(0, failed)
		return
	}
	for _, e := range es {
		t.Run(e, func(f TeleportFailure) {
			if f == "" {
				ok++
			} else {
				failed[f]++
			}
			if left--; left == 0 {
				done(ok, failed)
			}
		})
	}
}

func (t *Teleport) target(e plugin.Entity) (plugin.Location, TeleportFailure) {
	if t.loc != nil {
		if t.loc.World == nil {
			return plugin.Location{}, TeleportNullWorld
		}
		return *t.loc, ""
	}
	if t.dest == nil {
		return plugin.Location{}, TeleportNullDestination
	}
	ev := &MVTeleportDestinationEvent{Destination: t.dest, Teleportee: e, Teleporter: t.By}
	t.core.fire(ev)
	if ev.Cancelled() {
		return plugin.Location{}, TeleportCancelled
	}
	loc, ok := t.dest.Location(e)
	if !ok {
		return plugin.Location{}, TeleportNullLocation
	}
	if loc.World == nil {
		return plugin.Location{}, TeleportNullWorld
	}
	return loc, ""
}

// move teleports e with its riders and vehicle as the passenger mode says.
// MV: AsyncSafetyTeleporterAction.doAsyncTeleport
func (t *Teleport) move(e plugin.Entity, loc plugin.Location) TeleportFailure {
	if v, ok := e.Vehicle(); ok && t.Passenger.DismountVehicle {
		if t.Passenger.VehicleFollow {
			return t.withPassengers(v, loc, v.Passengers())
		}
		e.Dismount()
	}
	if ps := e.Passengers(); len(ps) > 0 && t.Passenger.DismountPassengers {
		for _, p := range ps {
			e.RemovePassenger(p)
		}
		if t.Passenger.PassengersFollow {
			return t.withPassengers(e, loc, ps)
		}
	}
	return t.single(e, loc)
}

func (t *Teleport) withPassengers(e plugin.Entity, loc plugin.Location, ps []plugin.Entity) TeleportFailure {
	for _, p := range ps {
		e.RemovePassenger(p)
	}
	fail := t.single(e, loc)
	for _, p := range ps {
		if f := t.single(p, loc); f != "" && fail == "" {
			fail = f
		}
	}
	if fail == "" {
		t.core.Server.Scheduler().RunTask(func() {
			for _, p := range ps {
				e.AddPassenger(p)
			}
		})
	}
	return fail
}

func (t *Teleport) single(e plugin.Entity, loc plugin.Location) TeleportFailure {
	p, isPlayer := e.(plugin.Player)
	if isPlayer {
		by := t.By
		if by == nil {
			by = p
		}
		t.core.teleportQueue[p.Name()] = by
		defer delete(t.core.teleportQueue, p.Name())
	}
	if !e.Teleport(loc, plugin.TeleportPlugin) {
		return TeleportFailed
	}
	if t.dest != nil {
		if v, ok := t.dest.Velocity(e); ok {
			t.core.Server.Scheduler().RunLater(1, func() { e.SetVelocity(v) })
		}
	}
	return ""
}
