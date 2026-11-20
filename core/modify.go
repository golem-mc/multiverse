package core

import "slices"

// ModifyActions are /mv modify's actions. MV: PropertyModifyAction
var ModifyActions = []string{"set", "add", "remove", "reset"}

// ModifyNeedsValue reports whether action takes a value: all but reset.
func ModifyNeedsValue(action string) bool { return action != "reset" }

// Modify changes a property by name as /mv modify does, and saves
// worlds.yml: set parses value, add and remove change an item of a list
// property, reset restores the default. MV:
// StringPropertyHandle.modifyPropertyString
func (w *World) Modify(action, name, value string) error {
	n, err := w.modifiable(action, name)
	if err != nil {
		return err
	}
	switch action {
	case "set":
		var v any
		if v, err = ParseString(n, value); err == nil {
			err = w.Props.Set(n, v)
		}
	case "add":
		err = w.Props.Add(n, value)
	case "remove":
		err = w.Props.Remove(n, value)
	case "reset":
		err = w.Props.Reset(n)
	}
	if err != nil {
		return err
	}
	return w.core.Worlds.Save()
}

// modifiable answers the visible node called name; add and remove take only
// list nodes.
func (w *World) modifiable(action, name string) (*Node, error) {
	n, ok := w.Props.Nodes().Find(name)
	if !ok || (action == "add" || action == "remove") && n.Kind != KindList {
		return nil, MsgError(M("mv-core.config.node.notfound", "{node}", name))
	}
	return n, nil
}

// ModifiableNames answers the properties action applies to. MV:
// StringPropertyHandle.getModifiablePropertyNames
func (w *World) ModifiableNames(action string) []string {
	switch action {
	case "set", "reset":
		return w.Props.Nodes().Names()
	case "add", "remove":
		return w.Props.Nodes().ListNames()
	}
	return nil
}

// SuggestModify completes the value of a property for action: remove
// offers the list's items. MV: StringPropertyHandle.getSuggestedPropertyValue
func (w *World) SuggestModify(action, name, input string) []string {
	n, err := w.modifiable(action, name)
	if err != nil {
		return nil
	}
	switch action {
	case "set", "add":
		return SuggestValues(n, input)
	case "remove":
		return slices.Clone(w.Props.List(n.Path))
	}
	return nil
}
