package app

import (
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/XiaTian-AC/faster-dooit/internal/dateparse"
)

// newInput returns a textinput configured with a prompt placeholder.
func newInput(placeholder string) textinput.Model {
	t := textinput.New()
	t.Placeholder = placeholder
	t.Prompt = ""
	t.CharLimit = 0
	return t
}

// StartEdit enters INSERT mode to edit the given field on the selected model.
// Fields: description, due, effort, urgency, recurrence.
func (m *Model) StartEdit(field string) tea.Cmd {
	m.editField = field
	m.input = newInput("")
	m.mode = ModeInsert

	switch field {
	case "description":
		if t := m.selectedTodo(); t != nil {
			m.input.SetValue(t.Description)
		} else if w := m.selectedWorkspaceByCursor(); w != nil {
			m.input.SetValue(w.Description)
		}
	case "due":
		if t := m.selectedTodo(); t != nil && t.Due != nil {
			m.input.SetValue(t.Due.Format("2006-01-02 15:04"))
		}
	case "effort":
		if t := m.selectedTodo(); t != nil {
			m.input.SetValue(itoa(t.Effort))
		}
	case "urgency":
		if t := m.selectedTodo(); t != nil {
			m.input.SetValue(itoa(t.Urgency))
		}
	case "recurrence":
		if t := m.selectedTodo(); t != nil && t.Recurrence != nil {
			m.input.SetValue(durationString(*t.Recurrence))
		}
	}
	m.bumpVersion()
	return nil
}

// ConfirmEdit applies the edited value back to the model and returns to
// NORMAL mode. Field-specific validation keeps the user in INSERT when the
// value is invalid (a deliberate improvement over the original, which exits
// editing and keeps the old value).
func (m *Model) ConfirmEdit() tea.Cmd {
	switch m.editField {
	case "description":
		m.applyDescription()
	case "due":
		if !m.applyDue() {
			return nil // stay in INSERT
		}
	case "effort":
		if !m.applyIntField("effort") {
			return nil
		}
	case "urgency":
		if !m.applyIntField("urgency") {
			return nil
		}
	case "recurrence":
		if !m.applyRecurrence() {
			return nil
		}
	}
	m.mode = ModeNormal
	m.editField = ""
	m.bumpVersion()
	return nil
}

// cancelMode returns the app to NORMAL without applying any pending input.
func (m *Model) cancelMode() {
	m.mode = ModeNormal
	m.editField = ""
	m.confirmCallback = nil
	m.bumpVersion()
}

// StartSearch opens the SEARCH overlay. Filtering lands in Task 6; the
// plumbing (prompt, enter to apply, escape to cancel) is live now.
func (m *Model) StartSearch() tea.Cmd {
	m.mode = ModeSearch
	m.input = newInput("/")
	m.bumpVersion()
	return nil
}

// StartSort opens the SORT overlay. Sorting semantics land in Task 7; the
// plumbing accepts a field name and returns to NORMAL.
func (m *Model) StartSort() tea.Cmd {
	m.mode = ModeSort
	m.input = newInput("sort field")
	m.bumpVersion()
	return nil
}

// StartConfirm opens the CONFIRM overlay with the given prompt and callback.
func (m *Model) StartConfirm() tea.Cmd {
	return m.StartConfirmPrompt("Are you sure? [y/N]", m.doDelete)
}

// StartConfirmPrompt opens a confirm dialog with an arbitrary prompt + callback.
func (m *Model) StartConfirmPrompt(prompt string, cb func() tea.Cmd) tea.Cmd {
	m.mode = ModeConfirm
	m.input = newInput(prompt)
	m.input.SetValue("")
	m.confirmCallback = cb
	m.bumpVersion()
	return nil
}

// Notify shows a transient message in the status/notification area.
func (m *Model) Notify(msg string, level string) tea.Cmd {
	m.notice = msg
	m.noticeLevel = level
	m.bumpVersion()
	return nil
}

// confirmYes executes the pending confirm callback.
func (m *Model) confirmYes() tea.Cmd {
	cb := m.confirmCallback
	m.confirmCallback = nil
	m.mode = ModeNormal
	m.bumpVersion()
	if cb != nil {
		return cb()
	}
	return nil
}

// confirmNo cancels the pending confirm.
func (m *Model) confirmNo() tea.Cmd {
	m.confirmCallback = nil
	m.mode = ModeNormal
	m.bumpVersion()
	return nil
}

// doDelete deletes the currently selected item (used by the confirm dialog).
func (m *Model) doDelete() tea.Cmd {
	if m.focus == PaneWorkspace {
		ws := m.selectedWorkspaceByCursor()
		if ws == nil || ws.IsRoot {
			return nil
		}
		if err := m.store.DeleteWorkspace(ws.ID); err != nil {
			return m.Notify("delete failed: "+err.Error(), "error")
		}
		m.RefreshFromStore()
		return nil
	}
	t := m.selectedTodo()
	if t == nil {
		return nil
	}
	if err := m.store.DeleteTodo(t.ID); err != nil {
		return m.Notify("delete failed: "+err.Error(), "error")
	}
	m.RefreshFromStore()
	return nil
}

// handleModeKey routes a KeyMsg through the active input overlay.
func (m *Model) handleModeKey(msg tea.KeyMsg) tea.Cmd {
	switch m.mode {
	case ModeInsert, ModeSearch, ModeSort:
		switch msg.Type {
		case tea.KeyEsc:
			m.cancelMode()
			return nil
		case tea.KeyEnter:
			return m.confirmMode()
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m.bumpVersion()
			return cmd
		}
	case ModeConfirm:
		// y/Y/Enter confirm; everything else (including escape) cancels —
		// matches the reference default-NO.
		if msg.Type == tea.KeyEnter {
			return m.confirmYes()
		}
		if len(msg.Runes) > 0 {
			switch msg.Runes[0] {
			case 'y', 'Y':
				return m.confirmYes()
			}
		}
		return m.confirmNo()
	}
	return nil
}

// confirmMode handles Enter in each input-driven mode.
func (m *Model) confirmMode() tea.Cmd {
	switch m.mode {
	case ModeInsert:
		return m.ConfirmEdit()
	case ModeSearch:
		m.filter = m.input.Value()
		m.mode = ModeNormal
		m.bumpVersion()
		return nil
	case ModeSort:
		m.mode = ModeNormal
		m.bumpVersion()
		return m.Notify("sort: "+m.input.Value(), "info")
	}
	return nil
}

// ----- field application helpers -----

func (m *Model) applyDescription() {
	val := m.input.Value()
	if m.focus == PaneTodo {
		if t := m.selectedTodo(); t != nil {
			t.Description = val
			m.store.SaveTodo(t)
		}
	} else {
		if w := m.selectedWorkspaceByCursor(); w != nil && !w.IsRoot {
			w.Description = val
			m.store.SaveWorkspace(w)
		}
	}
}

// applyDue parses the input as a date. On failure it notifies and returns
// false (the caller keeps the app in INSERT). An empty value clears the due.
func (m *Model) applyDue() bool {
	t := m.selectedTodo()
	if t == nil {
		return true
	}
	raw := m.input.Value()
	if raw == "" {
		t.Due = nil
		if err := m.store.SaveTodo(t); err != nil {
			m.Notify("save failed: "+err.Error(), "error")
		}
		return true
	}
	parsed, err := dateparse.Parse(raw, m.now())
	if err != nil {
		m.Notify("invalid date: "+err.Error(), "error")
		return false
	}
	t.Due = &parsed
	if err := m.store.SaveTodo(t); err != nil {
		m.Notify("save failed: "+err.Error(), "error")
	}
	return true
}

// applyIntField parses effort/urgency as a non-negative integer.
func (m *Model) applyIntField(field string) bool {
	t := m.selectedTodo()
	if t == nil {
		return true
	}
	raw := m.input.Value()
	if raw == "" {
		return true // keep current value
	}
	n, ok := atoiSafe(raw)
	if !ok {
		m.Notify(field+" must be a non-negative integer", "error")
		return false
	}
	switch field {
	case "effort":
		t.Effort = n
	case "urgency":
		t.Urgency = n
	}
	if err := m.store.SaveTodo(t); err != nil {
		m.Notify("save failed: "+err.Error(), "error")
	}
	return true
}

// applyRecurrence parses a duration token like "1d" / "2w" / "3h" / "30m".
func (m *Model) applyRecurrence() bool {
	t := m.selectedTodo()
	if t == nil {
		return true
	}
	raw := m.input.Value()
	if raw == "" {
		t.Recurrence = nil
		m.store.SaveTodo(t)
		return true
	}
	d, ok := parseDurationToken(raw)
	if !ok {
		m.Notify("invalid recurrence (use e.g. 1d, 2w, 3h, 30m)", "error")
		return false
	}
	t.Recurrence = &d
	// Setting a recurrence forces the todo pending (matches the original).
	t.Pending = true
	if err := m.store.SaveTodo(t); err != nil {
		m.Notify("save failed: "+err.Error(), "error")
	}
	return true
}

// now is a small seam so tests can control time.
func (m *Model) now() time.Time { return time.Now() }

// itoa is a small int-to-string helper.
func itoa(n int) string { return strconv.Itoa(n) }

// atoiSafe parses a non-negative integer, reporting success.
func atoiSafe(s string) (int, bool) {
	if s == "" {
		return 0, true
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// durationString renders a duration in a compact "1d 2h 3m" form.
func durationString(d time.Duration) string {
	if d < time.Minute {
		return "0m"
	}
	mins := int(d / time.Minute)
	out := ""
	if h := mins / 60; h > 0 {
		out += strconv.Itoa(h) + "h "
	}
	if m := mins % 60; m > 0 {
		out += strconv.Itoa(m) + "m"
	}
	if out == "" {
		return "0m"
	}
	return out
}

// parseDurationToken parses a single-token duration like "1d" / "2w" / "3h"
// / "30m". Matches the reference model_inputs.py grammar ^(\d+)[mhdw]$.
func parseDurationToken(s string) (time.Duration, bool) {
	if len(s) < 2 {
		return 0, false
	}
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n <= 0 {
		return 0, false
	}
	switch unit {
	case 'm':
		return time.Duration(n) * time.Minute, true
	case 'h':
		return time.Duration(n) * time.Hour, true
	case 'd':
		return time.Duration(n) * 24 * time.Hour, true
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, true
	}
	return 0, false
}
