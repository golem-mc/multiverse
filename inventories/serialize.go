package inventories

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/nbt"
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/multiverse/core"
)

// dataVersion is the world data version of the Minecraft release golem
// speaks, written with each item as Bukkit and Paper do.
const dataVersion = 5023

// obj is a JSON object that keeps its keys in order, as the Java side's
// LinkedHashMap does.
type obj []kv

type kv struct {
	k string
	v any
}

func (o obj) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, e := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := json.Marshal(e.k)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		v, err := marshalJSON(e.v)
		if err != nil {
			return nil, err
		}
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// marshalJSON writes v without escaping <, > and & as Java's JSON writer
// does not.
func marshalJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

// decodeJSON reads a JSON document with its numbers as json.Number.
func decodeJSON(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// javaDouble and javaFloat write numbers as Java's Double.toString and
// Float.toString do for ordinary magnitudes.
func javaDouble(f float64) json.Number { return json.Number(core.JavaDouble(f)) }

func javaFloat(f float32) json.Number {
	if math.IsInf(float64(f), 0) || math.IsNaN(float64(f)) {
		return "0.0"
	}
	s := strconv.FormatFloat(float64(f), 'f', -1, 32)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return json.Number(s)
}

// number reads a JSON number or a numeric string.
func number(raw any) (float64, bool) {
	switch v := raw.(type) {
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case float64:
		return v, true
	case int:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	}
	return 0, false
}

// encodeStat writes a stat as the string Java's toString gives.
func encodeStat(_ *Module, v any) any {
	switch x := v.(type) {
	case float64:
		return string(javaDouble(x))
	case float32:
		return string(javaFloat(x))
	case int:
		return strconv.Itoa(x)
	}
	return fmt.Sprint(v)
}

func decodeDouble(_ *Module, raw any) (any, bool) {
	f, ok := number(raw)
	return f, ok
}

func decodeFloat(_ *Module, raw any) (any, bool) {
	f, ok := number(raw)
	return float32(f), ok
}

func decodeInt(_ *Module, raw any) (any, bool) {
	if s, ok := raw.(string); ok {
		i, err := strconv.Atoi(strings.TrimSpace(s))
		return i, err == nil
	}
	f, ok := number(raw)
	return int(f), ok
}

// encodeSlots writes an inventory as Multiverse-Inventories does: an
// object of the slots that hold something, by slot number.
// MV: InventorySerializer.mapSlots
func encodeSlots(m *Module, v any) any {
	out := obj{}
	for i, s := range v.([]plugin.ItemStack) {
		if item := m.encodeItem(s); item != nil {
			out = append(out, kv{strconv.Itoa(i), item})
		}
	}
	return out
}

// decodeSlots reads an inventory of the size the config node at sizePath
// gives. MV: InventorySerializer.unmapSlots
func decodeSlots(sizePath string) func(m *Module, raw any) (any, bool) {
	return func(m *Module, raw any) (any, bool) {
		out := make([]plugin.ItemStack, max(m.Config.Int(sizePath), 0))
		slots, ok := raw.(map[string]any)
		if !ok {
			return out, true
		}
		for i := range out {
			if s, ok := m.decodeItem(slots[strconv.Itoa(i)]); ok {
				out[i] = s
			}
		}
		return out, true
	}
}

// encodeItem writes a stack: nil for an empty one, Paper's byte form with
// use-byte-serialization-for-inventory-data or when it has components
// Bukkit's map cannot carry, else Bukkit's ConfigurationSerializable map.
// MV: ItemStackConverter.serialize
func (m *Module) encodeItem(s plugin.ItemStack) any {
	if s.Empty() {
		return nil
	}
	text := m.Server.Items().Format(s)
	if m.Config.Bool("sharables.use-byte-serialization-for-inventory-data") || strings.Contains(text, "[") {
		data, err := itemBytes(text, s.Count)
		if err != nil {
			m.Log.Error("Could not byte serialize item stack", "error", err)
			return nil
		}
		return base64.StdEncoding.EncodeToString(data)
	}
	out := obj{{"==", "org.bukkit.inventory.ItemStack"}, {"v", dataVersion},
		{"type", strings.ToUpper(strings.TrimPrefix(s.Item.Name(), "minecraft:"))}}
	if s.Count != 1 {
		out = append(out, kv{"amount", s.Count})
	}
	return out
}

// itemBytes answers Paper's ItemStack.serializeAsBytes for a stack in
// /give's syntax: the item's saved compound with its DataVersion, gzipped.
func itemBytes(text string, count int) ([]byte, error) {
	id, rest, _ := strings.Cut(text, "[")
	components := nbt.Compound{}
	rest = strings.TrimSuffix(rest, "]")
	for rest != "" {
		if removed, ok := strings.CutPrefix(rest, "!"); ok {
			var key string
			key, rest, _ = strings.Cut(removed, ",")
			components["!"+key] = nbt.Compound{}
			continue
		}
		key, after, _ := strings.Cut(rest, "=")
		tag, n, err := nbt.ParseValueTag(after)
		if err != nil {
			return nil, err
		}
		components[key] = tag
		rest = strings.TrimPrefix(after[n:], ",")
	}
	root := nbt.Compound{"id": nbt.String(id), "count": nbt.Int(int32(count)), "DataVersion": nbt.Int(dataVersion)}
	if len(components) > 0 {
		root["components"] = components
	}
	data, err := nbt.Encode("", root)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// decodeItem reads a stack from Paper's byte form or Bukkit's map.
// MV: ItemStackConverter.deserialize
func (m *Module) decodeItem(raw any) (plugin.ItemStack, bool) {
	switch v := raw.(type) {
	case string:
		data, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return plugin.ItemStack{}, false
		}
		s, err := m.itemFromBytes(data)
		if err != nil {
			m.Log.Warn("Could not read item stack", "error", err)
			return plugin.ItemStack{}, false
		}
		return s, true
	case map[string]any:
		return m.itemFromMap(v)
	}
	return plugin.ItemStack{}, false
}

func (m *Module) itemFromBytes(data []byte) (plugin.ItemStack, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return plugin.ItemStack{}, err
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return plugin.ItemStack{}, err
	}
	_, tag, err := nbt.Decode(raw)
	if err != nil {
		return plugin.ItemStack{}, err
	}
	root, ok := nbt.CompoundView(tag)
	if !ok {
		return plugin.ItemStack{}, fmt.Errorf("not a compound")
	}
	id, _ := nbt.Get[nbt.String](root, "id")
	count := int32(1)
	if c, ok := root["count"]; ok {
		count, _ = nbt.IntValue(c)
	}
	var b strings.Builder
	b.WriteString(string(id))
	if comps, ok := nbt.CompoundView(root["components"]); ok && len(comps) > 0 {
		b.WriteByte('[')
		first := true
		for k, v := range comps {
			if !first {
				b.WriteByte(',')
			}
			first = false
			if strings.HasPrefix(k, "!") {
				b.WriteString(k)
				continue
			}
			b.WriteString(k + "=" + nbt.Stringify(v))
		}
		b.WriteByte(']')
	}
	return m.Server.Items().Parse(b.String(), int(count))
}

// itemFromMap reads Bukkit's ItemStack map: its type and amount, and of
// its meta the damage. Other meta Bukkit wrote is not read.
func (m *Module) itemFromMap(v map[string]any) (plugin.ItemStack, bool) {
	typ, _ := v["type"].(string)
	item, ok := registry.ItemByName("minecraft:" + strings.ToLower(typ))
	if !ok || item == registry.ItemAir {
		return plugin.ItemStack{}, false
	}
	count := 1
	if a, ok := number(v["amount"]); ok {
		count = int(a)
	}
	s := plugin.NewStack(item, count)
	if meta, ok := v["meta"].(map[string]any); ok {
		if d, ok := number(meta["Damage"]); ok && d > 0 {
			damage := int(d)
			s.Patch.Damage = &damage
		}
	}
	return s, !s.Empty()
}

// Loc is a saved location: a world by name, as Multiverse-Core's
// UnloadedWorldLocation, and for a respawn point what it is
// (RespawnLocation's respawnType: BED, ANCHOR or UNKNOWN).
type Loc struct {
	World      string
	X, Y, Z    float64
	Yaw, Pitch float32
	Respawn    string
}

func encodeLocation(_ *Module, v any) any {
	l := v.(*Loc)
	class := "UnloadedWorldLocation"
	if l.Respawn != "" {
		class = "RespawnLocation"
	}
	out := obj{{"==", class}}
	if l.World != "" {
		out = append(out, kv{"world", l.World})
	}
	out = append(out, kv{"x", javaDouble(l.X)}, kv{"y", javaDouble(l.Y)}, kv{"z", javaDouble(l.Z)},
		kv{"yaw", javaFloat(l.Yaw)}, kv{"pitch", javaFloat(l.Pitch)})
	if l.Respawn != "" {
		out = append(out, kv{"respawnType", l.Respawn})
	}
	return out
}

// decodeLocation reads a Bukkit Location, UnloadedWorldLocation or
// RespawnLocation map. MV: LocationSerializer
func decodeLocation(_ *Module, raw any) (any, bool) {
	v, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	l := &Loc{}
	l.World, _ = v["world"].(string)
	l.X, _ = number(v["x"])
	l.Y, _ = number(v["y"])
	l.Z, _ = number(v["z"])
	yaw, _ := number(v["yaw"])
	pitch, _ := number(v["pitch"])
	l.Yaw, l.Pitch = float32(yaw), float32(pitch)
	if v["=="] == "RespawnLocation" {
		l.Respawn = "UNKNOWN"
		if t, ok := v["respawnType"].(string); ok && (strings.EqualFold(t, "BED") || strings.EqualFold(t, "ANCHOR")) {
			l.Respawn = strings.ToUpper(t)
		}
	}
	return l, true
}

// encodeEffects writes the effects as Bukkit's PotionEffect maps.
func encodeEffects(m *Module, v any) any {
	out := []any{}
	for _, e := range v.([]plugin.EffectInstance) {
		key, ok := m.Server.Registries().KeyOf(e.Effect)
		if !ok {
			continue
		}
		out = append(out, obj{{"==", "PotionEffect"}, {"effect", key.String()}, {"duration", e.Duration},
			{"amplifier", e.Amplifier}, {"ambient", e.Ambient}, {"has-particles", e.Particles}, {"has-icon", e.Icon}})
	}
	return out
}

func decodeEffects(m *Module, raw any) (any, bool) {
	list, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	out := []plugin.EffectInstance{}
	for _, e := range list {
		v, ok := e.(map[string]any)
		if !ok {
			continue
		}
		name, _ := v["effect"].(string)
		key, err := plugin.ParseKey(strings.ToLower(name))
		if err != nil {
			continue
		}
		effect, ok := m.Server.Registries().Effect(key)
		if !ok {
			continue
		}
		duration, _ := number(v["duration"])
		amplifier, _ := number(v["amplifier"])
		flag := func(k string, def bool) bool {
			if b, ok := v[k].(bool); ok {
				return b
			}
			return def
		}
		out = append(out, plugin.EffectInstance{Effect: effect, Duration: int(duration), Amplifier: int(amplifier),
			Ambient: flag("ambient", false), Particles: flag("has-particles", true), Icon: flag("has-icon", true)})
	}
	return out, true
}
