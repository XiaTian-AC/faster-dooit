-- config.lua — faster-dooit default configuration.
--
-- This is YOUR config file: edit it in place. It mirrors the behaviour of
-- the original dooit default_config.py through a deliberately closed Lua
-- API subset (keys / layouts / formatter / bar / dashboard / subscribe /
-- timer / theme / vars / notify). It is NOT equivalent to the full dooit
-- Python API; api.css and the plugin_manager are intentionally absent.
--
-- Theme: pick a built-in preset, then optionally override individual colors.
-- Built-ins: nord, catppuccin_mocha, catppuccin_latte, dracula,
--            gruvbox_dark, solarized_light, tokyo_night
theme.name = "nord"

-- Override a color on top of the selected theme (optional):
-- theme.primary = "#8FBCBB"
-- theme.secondary = "#81A1C1"
-- theme.background = "#2E3440"
-- theme.background1 = "#3B4252"
-- theme.green = "#A3BE8C"
-- theme.yellow = "#EBCB8B"
-- theme.orange = "#D08770"
-- theme.red = "#BF616A"
-- theme.dim = "#4C566A"
-- theme.selection = "#3B4252"
-- theme.border_focused = "#8FBCBB"
-- theme.border_unfocused = "#4C566A"

-- Transparent background: set background to "transparent" to leave the
-- terminal's own background visible (no full-screen fill).
-- theme.background = "transparent"

-- Urgency colors for levels 1..5 (index 1 = urgency 1). Customize freely.
vars.urgency_colors = { "#A3BE8C", "#EBCB8B", "#D08770", "#BF616A", "#FF5C5C" }

-- Default tree-collapse depth: nodes at depth > collapse_depth start
-- collapsed. 0 = expand everything (default). Toggle any node with z / Z.
vars.collapse_depth = 0

-- Column layout for each pane.
layouts.workspace = { "description" }
layouts.todo = { "status", "description", "due", "effort", "recurrence", "urgency" }

-- Todo formatters. Each returns {text=..., style=...}; style is a color
-- keyword or hex, mapped to lipgloss by the renderer.
formatter.todos.status.add(function(status, model, theme)
  if status == "completed" then
    return { text = "x", style = theme.green }
  end
  if status == "overdue" then
    return { text = "!", style = theme.red }
  end
  return { text = "o", style = theme.yellow }
end)

formatter.todos.due.add(function(due, model, theme)
  if due == nil or due == "" then
    return { text = "", style = "" }
  end
  return { text = due, style = "" }
end)

formatter.todos.urgency.add(function(urgency, model, theme)
  if urgency == 0 then
    return { text = "", style = "" }
  end
  local colors = { theme.green, theme.yellow, theme.orange, theme.red }
  local color = colors[urgency] or theme.primary
  return { text = "!" .. urgency, style = color }
end)

formatter.todos.recurrence.add(function(recurrence, model, theme)
  if recurrence == nil or recurrence == "" then
    return { text = "", style = "" }
  end
  return { text = recurrence, style = theme.secondary }
end)

-- Status bar widgets: mode indicator, clock, user.
bar.set({
  function(value, model, theme)
    local mode = "NORMAL"
    return { text = " " .. mode .. " ", style = theme.primary }
  end,
  function(value, model, theme)
    return { text = " ", style = "" }
  end,
  function(value, model, theme)
    local now = now("%H:%M:%S")
    return { text = " " .. now .. " ", style = theme.secondary }
  end,
})

-- Dashboard shown in the right pane until a workspace is selected.
dashboard.set({
  "Welcome to Faster Dooit!",
  "",
  "If you're stuck, press '?' for help.",
})

-- Keybindings (mirrors the original defaults; chords are single strings).
keys.set("j", move_down)
keys.set("k", move_up)
keys.set("i", edit_description)
keys.set("d", edit_due)
keys.set("r", edit_recurrence)
keys.set("e", edit_effort)
keys.set("a", add_sibling)
keys.set("z", toggle_expand)
keys.set("Z", toggle_expand_parent)
keys.set("gg", go_to_top)
keys.set("G", go_to_bottom)
keys.set("A", add_child)
keys.set("J", shift_down)
keys.set("K", shift_up)
keys.set("xx", delete)
keys.set("y", copy_description)
keys.set("Y", copy_model)
keys.set("p", paste_below)
keys.set("P", paste_above)
keys.set("c", toggle_complete)
keys.set({ "=", "+" }, increase_urgency)
keys.set({ "-", "_" }, decrease_urgency)
keys.set("/", redraw)
keys.set("S", start_search)
keys.set("?", show_help)

-- Events / timers (plumbing is live; the app drives them in Task 6).
subscribe("ModeChanged", function(_, event)
  return event and event.mode or "NORMAL"
end)

timer(1, function()
  return theme.primary
end)
