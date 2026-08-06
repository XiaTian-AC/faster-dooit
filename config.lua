-- config.lua — faster-dooit default configuration.
--
-- This is YOUR config file: edit it in place. It mirrors the behaviour of
-- the original dooit default_config.py through a deliberately closed Lua
-- API subset (keys / layouts / formatter / bar / dashboard / subscribe /
-- timer / vars.theme / notify). It is NOT equivalent to the full dooit
-- Python API; api.css and the plugin_manager are intentionally absent.
--
-- Theme: pick a built-in preset, then optionally override individual colors.
-- Built-ins: nord, catppuccin_mocha, catppuccin_latte, dracula,
--            gruvbox_dark, solarized_light, tokyo_night
api.vars.theme.name = "nord"

-- Override a color on top of the selected theme (optional):
-- api.vars.theme.primary = "#8FBCBB"
-- api.vars.theme.secondary = "#81A1C1"
-- api.vars.theme.background = "#2E3440"
-- api.vars.theme.background1 = "#3B4252"
-- api.vars.theme.green = "#A3BE8C"
-- api.vars.theme.yellow = "#EBCB8B"
-- api.vars.theme.orange = "#D08770"
-- api.vars.theme.red = "#BF616A"
-- api.vars.theme.dim = "#4C566A"
-- api.vars.theme.selection = "#3B4252"
-- api.vars.theme.border_focused = "#8FBCBB"
-- api.vars.theme.border_unfocused = "#4C566A"

-- Transparent background: set background to "transparent" to leave the
-- terminal's own background visible (no full-screen fill).
-- api.vars.theme.background = "transparent"

-- Urgency colors for levels 1..5 (index 1 = urgency 1). Customize freely.
api.vars.urgency_colors = { "#A3BE8C", "#EBCB8B", "#D08770", "#BF616A", "#FF5C5C" }

-- Column layout for each pane.
api.layouts.workspace_layout = { "description" }
api.layouts.todo_layout = { "status", "description", "due", "effort", "recurrence", "urgency" }

-- Todo formatters. Each returns {text=..., style=...}; style is a color
-- keyword or hex, mapped to lipgloss by the renderer.
api.formatter.todos.status.add(function(status, model, theme)
  if status == "completed" then
    return { text = "x", style = theme.green }
  end
  if status == "overdue" then
    return { text = "!", style = theme.red }
  end
  return { text = "o", style = theme.yellow }
end)

api.formatter.todos.due.add(function(due, model, theme)
  if due == nil or due == "" then
    return { text = "", style = "" }
  end
  return { text = due, style = "" }
end)

api.formatter.todos.urgency.add(function(urgency, model, theme)
  if urgency == 0 then
    return { text = "", style = "" }
  end
  local colors = { theme.green, theme.yellow, theme.orange, theme.red }
  local color = colors[urgency] or theme.primary
  return { text = "!" .. urgency, style = color }
end)

api.formatter.todos.recurrence.add(function(recurrence, model, theme)
  if recurrence == nil or recurrence == "" then
    return { text = "", style = "" }
  end
  return { text = recurrence, style = theme.secondary }
end)

-- Status bar widgets: mode indicator, clock, user.
api.bar.set({
  function(api)
    local theme = api.vars.theme
    local mode = "NORMAL"
    return { text = " " .. mode .. " ", style = theme.primary }
  end,
  function(api)
    return { text = " ", style = "" }
  end,
  function(api)
    local now = api.now("%H:%M:%S")
    return { text = " " .. now .. " ", style = api.vars.theme.secondary }
  end,
})

-- Dashboard shown in the right pane until a workspace is selected.
api.dashboard.set({
  "Welcome to Faster Dooit!",
  "",
  "If you're stuck, press '?' for help.",
})

-- Keybindings (mirrors the original defaults; chords are single strings).
api.keys.set("j", api.move_down)
api.keys.set("k", api.move_up)
api.keys.set("i", api.edit_description)
api.keys.set("d", api.edit_due)
api.keys.set("r", api.edit_recurrence)
api.keys.set("e", api.edit_effort)
api.keys.set("a", api.add_sibling)
api.keys.set("z", api.toggle_expand)
api.keys.set("Z", api.toggle_expand_parent)
api.keys.set("gg", api.go_to_top)
api.keys.set("G", api.go_to_bottom)
api.keys.set("A", api.add_child)
api.keys.set("J", api.shift_down)
api.keys.set("K", api.shift_up)
api.keys.set("xx", api.delete)
api.keys.set("y", api.copy_description)
api.keys.set("Y", api.copy_model)
api.keys.set("p", api.paste_below)
api.keys.set("P", api.paste_above)
api.keys.set("c", api.toggle_complete)
api.keys.set({ "=", "+" }, api.increase_urgency)
api.keys.set({ "-", "_" }, api.decrease_urgency)
api.keys.set("/", api.redraw)
api.keys.set("S", api.start_search)
api.keys.set("?", api.show_help)

-- Events / timers (plumbing is live; the app drives them in Task 6).
subscribe("ModeChanged", function(api, event)
  return event and event.mode or "NORMAL"
end)

timer(1, function(api)
  return api.vars.theme.primary
end)
