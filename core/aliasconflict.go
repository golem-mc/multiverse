package core

import "strings"

// SendAliasConflicts tells is when w's alias is another world's alias or
// name, colours and case ignored. MV: AliasNameConflictChecker
func (ws *Worlds) SendAliasConflicts(w *World, is *Issuer) {
	alias := StripColor(w.Alias())
	if alias == "" {
		return
	}
	var aliases, names []*World
	for _, other := range ws.list {
		if other.Key() == w.Key() {
			continue
		}
		if strings.EqualFold(alias, StripColor(other.Alias())) {
			aliases = append(aliases, other)
		}
		if strings.EqualFold(alias, other.Name()) {
			names = append(names, other)
		}
	}
	if len(aliases) == 0 && len(names) == 0 {
		return
	}
	is.Error(M("mv-core.aliasnameconflict.detected", "{world}", w.Name(), "{alias}", w.ColourlessAlias()))
	for _, o := range aliases {
		is.Error(M("mv-core.aliasnameconflict.duplicatealias", "{world}", o.Name(), "{alias}", o.ColourlessAlias()))
	}
	for _, o := range names {
		is.Error(M("mv-core.aliasnameconflict.duplicateworldname", "{world}", o.Name()))
	}
}
