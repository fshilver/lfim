# Local-First Issue Manager (lfim)

A markdown-based local issue management CLI/TUI tool. Manage issues alongside your code repository without external SaaS dependencies.

## Features

- Terminal-based TUI interface
- Issues stored as Markdown with YAML frontmatter
- AI-powered issue analysis and implementation planning via Claude
- Git integration with automatic staging
- Status-based filtering (Active/All/Closed)

## Requirements

- Go 1.25+
- Claude API access (for AI features)

## Installation

```bash
# Install dependencies
make deps

# Build
make build

# Install to GOPATH/bin
make install
```

## Usage

```bash
# Run TUI (current directory)
lfim

# Specify project path
lfim --path /path/to/project

# Run in development mode
make run
```

## Issue Lifecycle

```
open → analyzed → planed → implemented → closed
  ↓        ↓           ↓
  └────────┴───────────┴──→ invalid
```

## Issue Types

| Type | Description |
|------|-------------|
| `feature` | New feature |
| `bug` | Bug fix |
| `refactor` | Code refactoring |

## TUI Shortcuts

| Key | Action | Description |
|-----|--------|-------------|
| `j/↓` | Down | Next issue |
| `k/↑` | Up | Previous issue |
| `n` | New | Create new issue |
| `a` | Analyze | AI analysis → analysis.md |
| `R` | Review | Review analysis.md with feedback |
| `p` | Plan | AI implementation plan → plan.md |
| `i` | Implement | Enter implementation mode |
| `c` | Close | Set status → closed |
| `d` | Discard | Set status → invalid |
| `e/↵` | Edit | Edit brief.md with $EDITOR |
| `f` | Filter | Toggle filter (Active/All) |
| `r` | Refresh | Refresh issue list |
| `q` | Quit | Exit |

## File Structure

```
project/
└── issues/
    ├── index.yaml           # Issue index
    └── 0001/
        ├── brief.md         # Issue description
        ├── analysis.md      # AI analysis result
        └── plan.md          # Implementation plan
```

### index.yaml

```yaml
issues:
  - id: '0001'
    title: 'Issue title'
    type: feature
    status: open
    created: 2025-12-19
```

### brief.md

```yaml
---
title: 'Issue title'
type: feature
status: open
date: 2025-12-19
---

Issue details...
```

## Tech Stack

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Styling
- [Cobra](https://github.com/spf13/cobra) - CLI framework

## License

MIT
