package core

import (
	"math"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/vault"
)

// EntryCheck checks whether a sender may enter worlds. MV: WorldEntryChecker
type EntryCheck struct {
	core   *Core
	sender plugin.CommandSender
}

// EntryFor answers the entry checker for sender. MV: WorldEntryCheckerProvider.forSender
func (c *Core) EntryFor(sender plugin.CommandSender) EntryCheck {
	return EntryCheck{core: c, sender: sender}
}

// CanEnter checks access, player limit, blacklist and entry fee for a trip
// from from (nil when unknown) to to; the error is the first failure.
// Paid reports that an item fee is due. MV: WorldEntryChecker.canEnterWorld
func (e EntryCheck) CanEnter(from, to *World) (pay bool, err error) {
	if err := e.Access(to); err != nil {
		return false, err
	}
	if err := e.PlayerLimit(to); err != nil {
		return false, err
	}
	if from != nil && slicesContainsFold(to.Props.List(PropWorldBlacklist), from.Name()) {
		return false, MsgError(M("mv-core.entrycheck.blacklisted", "{world}", from.AliasOrName()))
	}
	return e.Fee(to)
}

// CanStay checks access and player limit. MV: WorldEntryChecker.canStayInWorld
func (e EntryCheck) CanStay(w *World) error {
	if err := e.Access(w); err != nil {
		return err
	}
	return e.PlayerLimit(w)
}

// Access checks multiverse.access.<world> when enforce-access is on.
func (e EntryCheck) Access(w *World) error {
	if !e.core.CanAccess(e.sender, w) {
		return MsgError(M("mv-core.entrycheck.noworldaccess"))
	}
	return nil
}

// PlayerLimit checks the world's player-limit, which
// mv.bypass.playerlimit.<world> lifts.
func (e EntryCheck) PlayerLimit(w *World) error {
	limit := w.Props.Int(PropPlayerLimit)
	if limit <= -1 || e.sender.HasPermission(Concat(PermPlayerLimitBypass, w.Name())) {
		return nil
	}
	if limit > len(w.Players()) {
		return nil
	}
	return MsgError(M("mv-core.entrycheck.exceedplayerlimit"))
}

// Fee checks the entry fee: free for the console and with
// multiverse.exempt.<world>; paid in items or, for @vault-economy, through
// the vault.Economy service. MV: WorldEntryChecker.canPayEntryFee
func (e EntryCheck) Fee(w *World) (pay bool, err error) {
	price := w.Props.Double(PropEntryFeeAmount)
	if !w.Props.Bool(PropEntryFeeEnabled) || price == 0 {
		return false, nil
	}
	p, isPlayer := e.sender.(plugin.Player)
	if !isPlayer {
		return false, nil
	}
	if e.sender.HasPermission(Concat(PermWorldExempt, w.Name())) {
		return false, nil
	}
	notEnough := MsgError(M("mv-core.entrycheck.notenoughmoney", "{amount}", e.core.FormatPrice(price, w.Props.String(PropEntryFeeCurrency))))
	if w.Props.String(PropEntryFeeCurrency) == VaultEconomyCode {
		eco, ok := vault.LookupEconomy(e.core.Server)
		if !ok {
			return false, MsgError(M("mv-core.entrycheck.cannotpayentryfee"))
		}
		if price > 0 && !eco.Has(p.UUID(), price) {
			return false, notEnough
		}
		return true, nil
	}
	item, ok := currencyItem(w)
	if !ok {
		return false, MsgError(M("mv-core.entrycheck.cannotpayentryfee"))
	}
	if price > 0 && p.Inventory().Count(item) < int(price) {
		return false, notEnough
	}
	return true, nil
}

// currencyItem answers the item an item entry fee is paid in.
func currencyItem(w *World) (registry.Item, bool) {
	return itemCurrency(w.Props.String(PropEntryFeeCurrency))
}

func itemCurrency(cur string) (registry.Item, bool) {
	if cur == VaultEconomyCode || cur == "" {
		return 0, false
	}
	return registry.ItemByName("minecraft:" + strings.ToLower(cur))
}

// PayEntryFee charges p w's entry fee, or pays it out when negative.
func (c *Core) PayEntryFee(p plugin.Player, w *World) {
	c.PayFee(p, w.Props.Double(PropEntryFeeAmount), w.Props.String(PropEntryFeeCurrency))
}

// HasFunds reports whether p holds price in currency: items named by it,
// or with VaultEconomyCode money of the vault.Economy service; false when
// there is no way to pay. MV: MVEconomist.isPlayerWealthyEnough
func (c *Core) HasFunds(p plugin.Player, price float64, currency string) bool {
	if price <= 0 {
		return true
	}
	if currency == VaultEconomyCode {
		eco, ok := vault.LookupEconomy(c.Server)
		return ok && eco.Has(p.UUID(), price)
	}
	item, ok := itemCurrency(currency)
	return ok && p.Inventory().Count(item) >= int(price)
}

// PayFee charges p price in currency, or pays it out when negative, as
// HasFunds reads currency. MV: MVEconomist.payEntryFee, ItemEconomy, VaultHandler
func (c *Core) PayFee(p plugin.Player, price float64, currency string) {
	if currency == VaultEconomyCode {
		eco, ok := vault.LookupEconomy(c.Server)
		if !ok || price == 0 {
			return
		}
		is := c.Issuer(p)
		if price > 0 {
			if eco.Withdraw(p.UUID(), price).Success() {
				is.Info(M("mv-core.economy.vault.withdraw", "{price}", eco.Format(price)))
			}
			return
		}
		if eco.Deposit(p.UUID(), -price).Success() {
			is.Info(M("mv-core.economy.vault.deposit", "{price}", eco.Format(-price)))
		}
		return
	}
	item, ok := itemCurrency(currency)
	if !ok || price == 0 {
		return
	}
	n := int(math.Abs(price))
	formatted := c.FormatPrice(float64(n), currency)
	is := c.Issuer(p)
	if price > 0 {
		p.Inventory().Remove(plugin.ItemStack{Item: item, Count: n})
		is.Info(M("mv-core.economy.item.withdraw", "{price}", formatted))
		return
	}
	p.Inventory().Add(plugin.ItemStack{Item: item, Count: n})
	is.Info(M("mv-core.economy.item.deposit", "{price}", formatted))
}

func slicesContainsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// TeleportScope is whether a teleport moves the sender or someone else.
// MV: CorePermissionsChecker.Scope
type TeleportScope string

const (
	ScopeSelf  TeleportScope = "self"
	ScopeOther TeleportScope = "other"
)

// ScopeOf answers the scope of teleporter moving e.
func ScopeOf(teleporter plugin.CommandSender, e plugin.Entity) TeleportScope {
	if p, ok := teleporter.(plugin.Player); ok && p.UUID() == e.UUID() {
		return ScopeSelf
	}
	return ScopeOther
}

// scopesOf answers the scopes of teleporter moving all of es.
func scopesOf(teleporter plugin.CommandSender, es []plugin.Entity) []TeleportScope {
	p, ok := teleporter.(plugin.Player)
	if !ok {
		return []TeleportScope{ScopeOther}
	}
	var self, other bool
	for _, e := range es {
		if e.UUID() == p.UUID() {
			self = true
		} else {
			other = true
		}
	}
	var out []TeleportScope
	if self {
		out = append(out, ScopeSelf)
	}
	if other {
		out = append(out, ScopeOther)
	}
	return out
}

// CanTeleport reports whether teleporter may send every one of es to d:
// multiverse.teleport.<scope>.<id>, with use-finer-teleport-permissions
// .<suffix> too. MV: CorePermissionsChecker.checkTeleportPermission
func (c *Core) CanTeleport(teleporter plugin.CommandSender, es []plugin.Entity, d Destination) bool {
	suffix, _ := d.PermissionSuffix()
	for _, s := range scopesOf(teleporter, es) {
		if !c.hasTeleportPerm(teleporter, s, d.Type().ID(), suffix) {
			return false
		}
	}
	return true
}

// CanSuggest reports whether teleporter may be offered s for es.
// MV: CorePermissionsChecker.checkDestinationPacketPermission
func (c *Core) CanSuggest(teleporter plugin.CommandSender, es []plugin.Entity, s Suggestion) bool {
	for _, sc := range scopesOf(teleporter, es) {
		if !c.hasTeleportPerm(teleporter, sc, s.Type.ID(), s.PermSuffix) {
			return false
		}
	}
	return true
}

func (c *Core) hasTeleportPerm(sender plugin.CommandSender, s TeleportScope, id, suffix string) bool {
	if c.Config.Bool("teleport.use-finer-teleport-permissions") && suffix != "" {
		return sender.HasPermission(Concat(PermTeleport, string(s), id, suffix))
	}
	return sender.HasPermission(Concat(PermTeleport, string(s), id))
}

// HasAnyTeleport reports whether sender holds a teleport permission of one
// of scopes. MV: CorePermissionsChecker.hasAnyTeleportPermission
func (c *Core) HasAnyTeleport(sender plugin.CommandSender, scopes ...TeleportScope) bool {
	if len(scopes) == 0 {
		scopes = []TeleportScope{ScopeSelf, ScopeOther}
	}
	finer := c.Config.Bool("teleport.use-finer-teleport-permissions")
	for _, sc := range scopes {
		if !finer {
			for _, t := range c.Destinations.types {
				if c.hasTeleportPerm(sender, sc, t.ID(), "") {
					return true
				}
			}
			continue
		}
		for _, s := range c.Destinations.SuggestAll(sender, "") {
			if c.hasTeleportPerm(sender, sc, s.Type.ID(), s.PermSuffix) {
				return true
			}
		}
	}
	return false
}

// CanSpawn reports whether teleporter may send es to w's spawn:
// multiverse.core.spawn.<scope>[.<world>]. MV: CorePermissionsChecker.checkSpawnPermission
func (c *Core) CanSpawn(teleporter plugin.CommandSender, es []plugin.Entity, w *World) bool {
	for _, s := range scopesOf(teleporter, es) {
		if !c.hasSpawnPerm(teleporter, s, w) {
			return false
		}
	}
	return true
}

func (c *Core) hasSpawnPerm(sender plugin.CommandSender, s TeleportScope, w *World) bool {
	if c.Config.Bool("teleport.use-finer-teleport-permissions") && w != nil {
		return sender.HasPermission(Concat(PermSpawn, string(s), w.Name()))
	}
	return sender.HasPermission(Concat(PermSpawn, string(s)))
}

// HasAnySpawn reports whether sender holds a spawn permission of one of
// scopes. MV: CorePermissionsChecker.hasAnySpawnPermission
func (c *Core) HasAnySpawn(sender plugin.CommandSender, scopes ...TeleportScope) bool {
	if len(scopes) == 0 {
		scopes = []TeleportScope{ScopeSelf, ScopeOther}
	}
	for _, s := range scopes {
		if !c.Config.Bool("teleport.use-finer-teleport-permissions") {
			if c.hasSpawnPerm(sender, s, nil) {
				return true
			}
			continue
		}
		for _, w := range c.Worlds.Loaded() {
			if c.hasSpawnPerm(sender, s, w) {
				return true
			}
		}
	}
	return false
}
