package portals

import (
	"bytes"
	"encoding/binary"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/core"
)

// Action is a parsed portal action: what happens to whoever uses a portal.
// MV: ActionHandler
type Action interface {
	Type() ActionType
	// Run performs the action for e, which used pt.
	Run(pt *Portal, e plugin.Entity) error
	// Description answers what the action does for e, for /mvp info.
	Description(e plugin.Entity) core.Message
	// Serialise answers the action string portals.yml keeps.
	Serialise() string
}

// ActionType parses one kind of action. MV: ActionHandlerType
type ActionType interface {
	// Name is the action-type value: "multiverse-destination", "command",
	// "server".
	Name() string
	Parse(sender plugin.CommandSender, action string) (Action, error)
	// Suggest completes an action being typed.
	Suggest(sender plugin.CommandSender, input string) []string
}

// Actions is the action type registry. MV: ActionHandlerProvider
type Actions struct {
	p     *Portals
	types []ActionType
}

func newActions(p *Portals) *Actions {
	a := &Actions{p: p}
	a.Register(&destinationActionType{p}, &commandActionType{p}, newServerActionType(p))
	return a
}

// Register adds action types, replacing any of the same name.
func (a *Actions) Register(types ...ActionType) {
	for _, t := range types {
		a.types = slices.DeleteFunc(a.types, func(x ActionType) bool { return x.Name() == t.Name() })
		a.types = append(a.types, t)
	}
}

// Names answers the registered types' names, sorted.
func (a *Actions) Names() []string {
	out := make([]string, len(a.types))
	for i, t := range a.types {
		out[i] = t.Name()
	}
	slices.Sort(out)
	return out
}

// Type answers the action type called name. MV: ActionHandlerProvider.getHandlerType
func (a *Actions) Type(name string) (ActionType, error) {
	for _, t := range a.types {
		if t.Name() == name {
			return t, nil
		}
	}
	return nil, core.MsgError(core.M("mv-portals.action.unknown.type", "{name}", name,
		"{types}", strings.Join(a.Names(), ", ")))
}

// Parse parses action as an action of the type called typ, as sender.
// MV: ActionHandlerProvider.parseHandler
func (a *Actions) Parse(sender plugin.CommandSender, typ, action string) (Action, error) {
	t, err := a.Type(typ)
	if err != nil {
		return nil, err
	}
	return t.Parse(sender, action)
}

// destinationActionType sends whoever uses the portal to a Multiverse
// destination. MV: MultiverseDestinationActionHandlerType
type destinationActionType struct{ p *Portals }

func (*destinationActionType) Name() string { return "multiverse-destination" }

func (t *destinationActionType) Parse(sender plugin.CommandSender, action string) (Action, error) {
	if action == "" {
		return nil, core.MsgError(core.M("mv-portals.action.multiversedestination.invalid"))
	}
	d, err := t.p.Core.Destinations.Parse(sender, action)
	if err != nil {
		return nil, err
	}
	return &destinationAction{typ: t, dest: d}, nil
}

func (t *destinationActionType) Suggest(sender plugin.CommandSender, input string) []string {
	return t.p.Core.Destinations.Suggest(sender, input)
}

type destinationAction struct {
	typ  *destinationActionType
	dest core.Destination
}

func (a *destinationAction) Type() ActionType { return a.typ }

// Run teleports e: a vehicle, or anything through a portal that takes
// non-players, with its riders and vehicle; a player alone otherwise.
// MV: MultiverseDestinationActionHandler.runAction
func (a *destinationAction) Run(pt *Portal, e plugin.Entity) error {
	c := a.typ.p.Core
	tp := c.TeleportTo(a.dest)
	tp.Safety = pt.CheckDestinationSafety() && a.dest.CheckSafety()
	tp.Passenger = core.PassengerModes["dismount_vehicle"]
	if _, isVehicle := e.(plugin.Vehicle); isVehicle || pt.TeleportNonPlayers() {
		tp.Passenger = core.PassengerModes["retain_all"]
	}
	tp.Run(e, func(f core.TeleportFailure) {
		if f != "" {
			a.typ.p.Log.Warn("Failed to teleport entity '" + entityName(e) + "' to destination '" +
				core.DestinationString(a.dest) + "'. Reason: " + string(f))
		}
	})
	return nil
}

func (a *destinationAction) Description(plugin.Entity) core.Message {
	return core.M("mv-portals.action.multiversedestination.description", "{destination}", core.DestinationString(a.dest))
}

func (a *destinationAction) Serialise() string { return core.DestinationString(a.dest) }

// commandActionType runs a command: "op:<command>" as an operator,
// "console:<command>" from the console, anything else as whoever used the
// portal. MV: CommandActionHandlerType, CommandRunner
type commandActionType struct{ p *Portals }

func (*commandActionType) Name() string { return "command" }

func (t *commandActionType) Parse(_ plugin.CommandSender, action string) (Action, error) {
	if action == "" {
		return nil, core.MsgError(core.M("mv-portals.action.command.invalid"))
	}
	a := &commandAction{typ: t, raw: action, cmd: action, mode: "self"}
	if mode, cmd, ok := strings.Cut(action, ":"); ok && (mode == "op" || mode == "console") {
		a.mode, a.cmd = mode, cmd
	}
	return a, nil
}

func (*commandActionType) Suggest(plugin.CommandSender, string) []string {
	return []string{"op:", "console:"}
}

type commandAction struct {
	typ            *commandActionType
	raw, cmd, mode string
}

func (a *commandAction) Type() ActionType { return a.typ }

// command fills in %world% and, for a player, %player%.
// MV: CommandRunner.parseCmdStrPlaceholders
func (a *commandAction) command(e plugin.Entity) string {
	cmd := a.cmd
	if e == nil {
		return cmd
	}
	if w, ok := a.typ.p.Core.Worlds.Of(e.World()); ok {
		cmd = strings.ReplaceAll(cmd, "%world%", w.Name())
	}
	if p, ok := e.(plugin.Player); ok {
		cmd = strings.ReplaceAll(cmd, "%player%", p.Name())
	}
	return cmd
}

// Run runs the command. A command run as whoever used the portal runs only
// for a player: golem's other entities cannot run commands.
func (a *commandAction) Run(_ *Portal, e plugin.Entity) error {
	srv := a.typ.p.Server
	cmd := a.command(e)
	switch a.mode {
	case "console":
		_, _ = srv.Dispatch(srv.Console(), cmd)
	case "op":
		p, ok := e.(plugin.Player)
		if !ok {
			return nil
		}
		// MV: CommandRunner.Op makes a non-operator one for the command.
		if level := p.OpLevel(); level == 0 {
			p.SetOpLevel(4)
			defer p.SetOpLevel(0)
		}
		_, _ = srv.Dispatch(p, cmd)
	default:
		if p, ok := e.(plugin.Player); ok {
			_, _ = srv.Dispatch(p, cmd)
		}
	}
	return nil
}

func (a *commandAction) Description(e plugin.Entity) core.Message {
	key := map[string]string{"self": "mv-portals.action.command.self", "op": "mv-portals.action.command.operator",
		"console": "mv-portals.action.command.console"}[a.mode]
	return core.M(key, "{command}", a.command(e))
}

func (a *commandAction) Serialise() string { return a.raw }

// bungeeChannel is BungeeCord's plugin channel.
var bungeeChannel = plugin.Key{Namespace: "bungeecord", Path: "main"}

// serverActionType sends a player to another server behind a BungeeCord
// proxy. MV: ServerActionHandlerType, BungeeServerList
type serverActionType struct {
	p *Portals
	// servers are the proxy's server names, asked for once a player is on.
	servers []string
	asked   bool
}

func newServerActionType(p *Portals) *serverActionType {
	t := &serverActionType{p: p}
	p.Server.Messenger().Listen(bungeeChannel, func(_ plugin.Player, data []byte) {
		r := bytes.NewReader(data)
		if sub, ok := readUTF(r); ok && sub == "GetServers" {
			if list, ok := readUTF(r); ok {
				t.servers = strings.Split(list, ", ")
			}
		}
	})
	plugin.Listen(p.Server, func(*plugin.PlayerJoinEvent) {
		if t.asked {
			return
		}
		t.asked = true
		// Asked a little later, once the player is fully in.
		p.Server.Scheduler().RunLater(10, t.askServers)
	})
	plugin.Listen(p.Server, func(*core.MVConfigReloadEvent) { t.askServers() })
	return t
}

// askServers asks the proxy for its server list through any player.
func (t *serverActionType) askServers() {
	ps := t.p.Server.Players()
	if len(ps) == 0 {
		return
	}
	var b bytes.Buffer
	writeUTF(&b, "GetServers")
	t.p.Server.Messenger().Send(ps[0], bungeeChannel, b.Bytes())
}

func (*serverActionType) Name() string { return "server" }

func (t *serverActionType) Parse(_ plugin.CommandSender, action string) (Action, error) {
	if action == "" {
		return nil, core.MsgError(core.M("mv-portals.action.server.invalid"))
	}
	return &serverAction{typ: t, server: action}, nil
}

func (t *serverActionType) Suggest(plugin.CommandSender, string) []string {
	return slices.Clone(t.servers)
}

type serverAction struct {
	typ    *serverActionType
	server string
}

func (a *serverAction) Type() ActionType { return a.typ }

func (a *serverAction) Run(_ *Portal, e plugin.Entity) error {
	p, ok := e.(plugin.Player)
	if !ok {
		return core.MsgError(core.M("mv-portals.action.server.playersonly"))
	}
	var b bytes.Buffer
	writeUTF(&b, "Connect")
	writeUTF(&b, a.server)
	p.SendPluginMessage(bungeeChannel, b.Bytes())
	return nil
}

func (a *serverAction) Description(plugin.Entity) core.Message {
	return core.M("mv-portals.action.server.description", "{name}", a.server)
}

func (a *serverAction) Serialise() string { return a.server }

// writeUTF writes s as Java's DataOutput.writeUTF does: a big-endian
// length, then modified UTF-8 (NUL and each UTF-16 unit of a
// supplementary character in their own multi-byte forms).
func writeUTF(b *bytes.Buffer, s string) {
	var enc []byte
	for _, u := range utf16.Encode([]rune(s)) {
		switch {
		case u >= 1 && u <= 0x7f:
			enc = append(enc, byte(u))
		case u <= 0x7ff:
			enc = append(enc, byte(0xc0|u>>6), byte(0x80|u&0x3f))
		default:
			enc = append(enc, byte(0xe0|u>>12), byte(0x80|(u>>6)&0x3f), byte(0x80|u&0x3f))
		}
	}
	_ = binary.Write(b, binary.BigEndian, uint16(len(enc)))
	b.Write(enc)
}

// readUTF reads what writeUTF writes.
func readUTF(r *bytes.Reader) (string, bool) {
	var n uint16
	if binary.Read(r, binary.BigEndian, &n) != nil || int(n) > r.Len() {
		return "", false
	}
	buf := make([]byte, n)
	_, _ = r.Read(buf)
	var units []uint16
	for i := 0; i < len(buf); {
		c := buf[i]
		switch {
		case c < 0x80:
			units = append(units, uint16(c))
			i++
		case c&0xe0 == 0xc0 && i+1 < len(buf):
			units = append(units, uint16(c&0x1f)<<6|uint16(buf[i+1]&0x3f))
			i += 2
		case i+2 < len(buf):
			units = append(units, uint16(c&0x0f)<<12|uint16(buf[i+1]&0x3f)<<6|uint16(buf[i+2]&0x3f))
			i += 3
		default:
			return "", false
		}
	}
	return string(utf16.Decode(units)), true
}

// entityName answers a name for logs.
func entityName(e plugin.Entity) string {
	if p, ok := e.(plugin.Player); ok {
		return p.Name()
	}
	return e.Type().Name()
}
