package core

import (
	"errors"
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// MessageError is an error that carries a localizable message, shown to
// the sender in their language. MV: MultiverseException, failure reasons
type MessageError struct{ Msg Message }

// MsgError wraps m as an error.
func MsgError(m Message) error { return &MessageError{Msg: m} }

func (e *MessageError) Error() string {
	return StripColor(english.Format(nil, e.Msg))
}

// english formats messages for logs and Error strings.
var english = newLocales()

// ErrorMessage answers the message an error carries, or its text as a
// literal.
func ErrorMessage(err error) Message {
	if me, ok := errors.AsType[*MessageError](err); ok {
		return me.Msg
	}
	return Lit("{error}", "{error}", err.Error())
}

// MessageKind is how a message is coloured before its own codes, as ACF's
// message types are. MV: ACF MessageType
type MessageKind uint8

const (
	// Plain adds no colour.
	Plain MessageKind = iota
	// Info is blue, ACF's INFO.
	Info
	// Error is red, ACF's ERROR.
	Error
	// Syntax is yellow, ACF's SYNTAX.
	Syntax
)

var kindColour = [...]string{Plain: "", Info: "§9", Error: "§c", Syntax: "§e"}

// Issuer is a command sender with Multiverse's message helpers. MV:
// MVCommandIssuer
type Issuer struct {
	Sender plugin.CommandSender
	core   *Core
}

// Issuer wraps sender.
func (c *Core) Issuer(sender plugin.CommandSender) *Issuer {
	return &Issuer{Sender: sender, core: c}
}

// Player answers the sender as a player.
func (i *Issuer) Player() (plugin.Player, bool) {
	p, ok := i.Sender.(plugin.Player)
	return p, ok
}

// IsPlayer reports whether the sender is a player.
func (i *Issuer) IsPlayer() bool { _, ok := i.Player(); return ok }

// IsConsole reports whether the sender is the server console.
func (i *Issuer) IsConsole() bool {
	return i.Sender == i.core.Server.Console() || (!i.IsPlayer() && i.Sender.Name() == i.core.Server.Console().Name())
}

// Has reports whether the sender has node.
func (i *Issuer) Has(node string) bool { return i.Sender.HasPermission(node) }

// Format renders m in the sender's language.
func (i *Issuer) Format(m Message) string { return i.core.Locales.Format(i.Sender, m) }

// Send sends m with no added colour.
func (i *Issuer) Send(m Message) { i.SendKind(Plain, m) }

// Info sends m in ACF's info colour.
func (i *Issuer) Info(m Message) { i.SendKind(Info, m) }

// Error sends m in ACF's error colour.
func (i *Issuer) Error(m Message) { i.SendKind(Error, m) }

// Fail sends err's message as an error.
func (i *Issuer) Fail(err error) { i.Error(ErrorMessage(err)) }

// SendKind sends m prefixed with kind's colour, one chat line per line of
// the message, as ACF splits on newlines.
func (i *Issuer) SendKind(kind MessageKind, m Message) {
	i.SendRaw(kindColour[kind] + i.Format(m))
}

// SendRaw sends a string with '&' or '§' colour codes, one line each.
func (i *Issuer) SendRaw(s string) {
	for line := range strings.SplitSeq(Colorize(s), "\n") {
		i.Sender.SendMessage(Legacy(line))
	}
}
