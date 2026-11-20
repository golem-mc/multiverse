package portals

import (
	"fmt"
	"strconv"
	"strings"
)

// versionOf answers a file's "version", 0 when it has none.
func versionOf(m map[string]any) float64 {
	switch v := m["version"].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

// movePath moves the value at from to to, when there is one.
// MV: MoveMigratorAction
func movePath(m map[string]any, from, to string) {
	if v, ok := deletePath(m, from); ok && v != nil {
		setPath(m, to, v)
	}
}

// setPath sets a dotted path, replacing whatever is in the way.
func setPath(m map[string]any, path string, v any) {
	parts := strings.Split(path, ".")
	for _, p := range parts[:len(parts)-1] {
		sub, ok := m[p].(map[string]any)
		if !ok {
			sub = map[string]any{}
			m[p] = sub
		}
		m = sub
	}
	m[parts[len(parts)-1]] = v
}

func deletePath(m map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	for _, p := range parts[:len(parts)-1] {
		sub, ok := m[p].(map[string]any)
		if !ok {
			return nil, false
		}
		m = sub
	}
	v, ok := m[parts[len(parts)-1]]
	delete(m, parts[len(parts)-1])
	return v, ok
}

// stringAt answers the scalar at a top-level key as a string.
func stringAt(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}
	if _, isMap := v.(map[string]any); isMap {
		return "", false
	}
	return fmt.Sprint(v), true
}
