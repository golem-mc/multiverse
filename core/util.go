package core

import (
	"crypto/md5"
	"math"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/vault"
)

// CanAccess reports whether sender may enter w: always, unless
// enforce-access is on, then with multiverse.access.<world>.
// MV: WorldEntryChecker.canAccessWorld
func (c *Core) CanAccess(sender plugin.CommandSender, w *World) bool {
	if !c.Config.Bool("world.enforce-access") {
		return true
	}
	return sender.HasPermission(Concat(PermWorldAccess, w.Name()))
}

// FormatPrice writes an entry fee: the economy's format for
// @vault-economy, else "<amount> <ITEM>". MV: MVEconomist.formatPrice
func (c *Core) FormatPrice(amount float64, currency string) string {
	if currency == VaultEconomyCode {
		if eco, ok := vault.LookupEconomy(c.Server); ok {
			return eco.Format(amount)
		}
		return ""
	}
	if currency == "" {
		return ""
	}
	return JavaDouble(amount) + " " + strings.ToUpper(currency)
}

// DecimalFormat writes f as Java's new DecimalFormat() with at most two
// fraction digits does: grouped thousands, no trailing zeros.
func DecimalFormat(f float64) string {
	f = math.RoundToEven(f*100) / 100
	s := strconv.FormatFloat(math.Abs(f), 'f', -1, 64)
	whole, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	if f < 0 {
		b.WriteByte('-')
	}
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if frac != "" {
		b.WriteString("." + frac)
	}
	return b.String()
}

// StrCoords writes a location as Multiverse's LocationManipulation.strCoords.
func StrCoords(x, y, z float64, pitch, yaw float32) string {
	return "§fX: §b" + DecimalFormat(x) + " §fY: §b" + DecimalFormat(y) + " §fZ: §b" + DecimalFormat(z) +
		" §fP: §6" + DecimalFormat(float64(pitch)) + " §fY: §6" + DecimalFormat(float64(yaw)) + " "
}

// NameUUID answers the version 3 UUID of name, as Java's
// UUID.nameUUIDFromBytes.
func NameUUID(name string) plugin.UUID {
	sum := md5.Sum([]byte(name))
	sum[6] = sum[6]&0x0f | 0x30
	sum[8] = sum[8]&0x3f | 0x80
	return plugin.UUID(sum)
}
