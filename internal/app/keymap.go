package app

// keyManager holds an in-flight key buffer and matches it against the
// binding set. It mirrors the prefix-matching state machine in the original
// dooit keys.py: no timeout, escape clears, dead-end key presses are
// swallowed (so pressing `g` then `k` discards the `k`).
//
// Bindings are a map of rune → either an action name (terminal) or a
// sub-table (continuation). Chords nest naturally.
type keyManager struct {
	buffer string
	table  map[string]any
}

// newKeyManager(bindings map[string]any) *keyManager {
func newKeyManager(bindings map[string]any) *keyManager {
	return &keyManager{table: bindings}
}

// bindingsFromLua converts the flat Lua key map (key → action name) into the
// keyManager's nested table: single-char keys map to an action string, chords
// nest into sub-tables ("gg" → {"g": {"g": "go_to_top"}}).
func bindingsFromLua(keys map[string]string) map[string]any {
	table := map[string]any{}
	for key, action := range keys {
		cur := table
		for i := 0; i < len(key); i++ {
			k := string(key[i])
			last := i == len(key)-1
			if last {
				cur[k] = action
				continue
			}
			sub, ok := cur[k].(map[string]any)
			if !ok {
				sub = map[string]any{}
				cur[k] = sub
			}
			cur = sub
		}
	}
	return table
}

// feed consumes one rune and returns the resolved action name, or "" if
// the buffer is still a prefix (or the chain ended in a dead end).
func (k *keyManager) feed(r rune) string {
	next := string(r)

	// If we have a buffer, descend into the sub-table at that key.
	var cur map[string]any
	if k.buffer == "" {
		cur = k.table
	} else {
		v, ok := k.table[k.buffer]
		if !ok {
			// buffer references a missing continuation; treat as dead-end.
			k.buffer = ""
			return ""
		}
		sub, ok := v.(map[string]any)
		if !ok {
			// we were already at a terminal — first reset to root.
			k.buffer = ""
			cur = k.table
		} else {
			cur = sub
		}
	}

	v, ok := cur[next]
	if !ok {
		// dead end
		k.buffer = ""
		return ""
	}
	switch t := v.(type) {
	case string:
		// terminal action
		k.buffer = ""
		return t
	case map[string]any:
		// continuation prefix
		if k.buffer == "" {
			k.buffer = next
		} else {
			k.buffer = k.buffer + next
		}
		return ""
	default:
		k.buffer = ""
		return ""
	}
}

// escape clears any pending chord buffer.
func (k *keyManager) escape() {
	k.buffer = ""
}

// defaultKeyBindings returns the plan's default keymap.
//
// Ctrl+key actions are dispatched from Update by inspecting KeyMsg.Type +
// KeyMsg.Ctrl, not through this table.
func defaultKeyBindings() map[string]any {
	return map[string]any{
		// single keys
		"j":   "move_down",
		"k":   "move_up",
		"G":   "go_to_bottom",
		"i":   "edit_description",
		"d":   "edit_due",
		"r":   "edit_recurrence",
		"e":   "edit_effort",
		"a":   "add_sibling",
		"A":   "add_child",
		"z":   "toggle_expand",
		"Z":   "toggle_expand_parent",
		"J":   "shift_down",
		"K":   "shift_up",
		"y":   "copy_description",
		"Y":   "copy_model",
		"p":   "paste_below",
		"P":   "paste_above",
		"c":   "toggle_complete",
		"=":   "increase_urgency",
		"+":   "increase_urgency",
		"-":   "decrease_urgency",
		"_":   "decrease_urgency",
		"/":   "redraw",
		"S":   "start_search",
		"?":   "show_help",
		"tab": "switch_focus",
		"h":   "switch_focus",
		"l":   "switch_focus",
		"\r":  "enter_edit_description",

		// chords: nested map {next-rune -> action or sub-map}
		"g": map[string]any{
			"g": "go_to_top",
		},
		"x": map[string]any{
			"x": "delete",
		},
	}
}
