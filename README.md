# klonch

A terminal-based task manager built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- **Task Management** - Create, edit, complete, and delete tasks
- **Projects** - Organize tasks into colored projects
- **Tags** - Add multiple tags to tasks for flexible categorization
- **Subtasks** - Single-level nesting for breaking down tasks
- **Dependencies** - Block tasks until dependencies are complete
- **Priorities** - Low, medium, high, and urgent levels
- **Time Tracking** - Manual logging and pomodoro timer
- **Multiple Views** - List, Kanban, Eisenhower matrix, Calendar, Focus, Stats
- **Filtering** - Filter by project, tags, or text search
- **Themes** - Nord, Dracula, Gruvbox, Catppuccin

## Installation

```bash
go install github.com/dori/klonch/cmd/klonch@latest
```

Or build from source:

```bash
git clone https://github.com/dori/klonch.git
cd klonch
go build -o klonch ./cmd/klonch
```

## Usage

### Quick Add

```bash
# Add a task
klonch add "Review pull request"

# Add with project, tags, priority, and due date
klonch add "Fix login bug #work @urgent !high due:tomorrow"
```

**Quick Add Syntax:**
| Prefix | Example | Description |
|--------|---------|-------------|
| `#` | `#work` | Assign to project (creates if needed) |
| `@` | `@urgent` | Add tag |
| `!` | `!high` | Set priority (low/medium/high/urgent) |
| `due:` | `due:tomorrow` | Set due date |

### Interactive TUI

```bash
# Start the TUI
klonch

# Start with a specific view
klonch --view kanban

# Start with a specific theme
klonch --theme dracula
```

## Keyboard Shortcuts

### Navigation

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `g` | Go to top |
| `G` | Go to bottom |
| `Scroll` | Mouse wheel to move cursor |
| `X` | Toggle mouse (for copy-paste) |

### Task Actions

| Key | Action |
|-----|--------|
| `a` | Add new task |
| `s` | Add subtask (sibling if on subtask) |
| `S` | Add child subtask (nested) |
| `Enter` | Edit task |
| `Tab` | Toggle done |
| `d` | Delete task |
| `p` | Cycle priority |
| `m` | Move to project |
| `t` | Toggle tag on task |
| `o` | Toggle expand subtasks |
| `E` | Expand all subtasks |
| `C` | Collapse all subtasks |
| `f` | Focus on task |

### Filtering

| Key | Action |
|-----|--------|
| `/` | Text search |
| `M` | Filter by project |
| `T` | Filter by tags |
| `Esc` | Clear filters / collapse subtasks |

### Selection

| Key | Action |
|-----|--------|
| `Space` | Toggle selection |
| `V` | Select all |

### Other

| Key | Action |
|-----|--------|
| `:` | Open command palette |
| `?` | Show help |
| `q` | Quit |

## Commands

Access commands via `:` (command palette).

### Task Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `due <date>` | `d` | Set due date (e.g., `due tomorrow`, `due friday`) |
| `priority <level>` | `pri`, `p` | Set priority (low/medium/high/urgent) |
| `tag <name>` | `t` | Add tag to task |
| `project <name>` | `proj`, `mv` | Move to project |
| `done` | `complete` | Toggle done status |
| `archive` | `arch` | Archive task |
| `delete` | `del`, `rm` | Delete task |

### Filter Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `filter <text>` | `f` | Text search filter |
| `filterproject` | `fp` | Filter by project |
| `filtertag` | `ft` | Filter by tags |
| `clear` | | Clear all filters |

### Management Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `newproject <name>` | `np` | Create new project |
| `newtag <name>` | `nt` | Create new tag |
| `projects` | `lsp` | List all projects |
| `tags` | `lst` | List all tags |
| `theme <name>` | | Change theme |
| `sort <field>` | | Sort by priority/due/title/status |

### Time Tracking

| Command | Aliases | Description |
|---------|---------|-------------|
| `starttime` | `start`, `track` | Start time tracking |
| `stoptime` | `stop` | Stop time tracking |
| `addtime <duration>` | `logtime` | Log time (e.g., `30m`, `1h30m`) |

## Views

Switch views using the number keys or command palette:

| Key | View |
|-----|------|
| `1` | List |
| `2` | Kanban |
| `3` | Eisenhower Matrix |
| `4` | Calendar |
| `5` | Focus Mode |
| `6` | Stats |

## Data Storage

Data is stored in `~/.local/share/klonch/klonch.db` (SQLite).

## Themes

Available themes:
- `nord` (default)
- `dracula`
- `gruvbox`
- `catppuccin`

Change with `:theme <name>` or start with `--theme <name>`.

## License

MIT
