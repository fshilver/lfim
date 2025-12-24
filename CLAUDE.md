# Local-First Issue Manager

## Overview

Markdown 기반 로컬 이슈 관리 CLI/TUI 도구. 외부 SaaS 의존성 없이 issues를 코드 저장소에 함께 관리.

## Tech Stack

- Go 1.25+
- TUI: Bubble Tea (charmbracelet/bubbletea)
- CLI: Cobra (spf13/cobra)
- Styling: Lipgloss (charmbracelet/lipgloss)
- CLI 명령어: `lfim`

## Project Structure

```
.
├── cmd/
│   └── lfim/main.go         # CLI 진입점
├── internal/
│   ├── model/
│   │   ├── issue.go         # Issue, IssueStatus, IssueType
│   │   └── index.go         # Index 구조체
│   ├── storage/
│   │   ├── storage.go       # 파일 읽기/쓰기
│   │   ├── frontmatter.go   # YAML frontmatter 파싱
│   │   └── git.go           # Git 연동
│   ├── claude/
│   │   ├── client.go        # Claude API 클라이언트
│   │   └── prompts.go       # AI 프롬프트
│   └── tui/
│       ├── app.go           # Bubble Tea App
│       ├── keys.go          # 키바인딩 정의
│       └── styles.go        # Lipgloss 스타일
├── go.mod
├── go.sum
└── Makefile
```

## Issue Lifecycle

```
open → analyzed → implemented → closed
  ↓        ↓           ↓
  └────────┴───────────┴──→ invalid
```

## Issue Types

- `feature`: 새 기능
- `bug`: 버그 수정
- `refactor`: 리팩토링

## File Formats

### index.yaml

```yaml
issues:
  - id: '0001'
    title: '이슈 제목'
    type: feature
    status: open
    created: 2025-12-19
```

### brief.md (YAML frontmatter)

```yaml
---
title: '이슈 제목'
type: feature
status: open
date: 2025-12-19
---
이슈 상세 내용...
```

## TUI Shortcuts

| Key     | Action  | Description                |
| ------- | ------- | -------------------------- |
| `j/↓`   | Down    | 다음 이슈                  |
| `k/↑`   | Up      | 이전 이슈                  |
| `n`     | New     | 새 이슈 생성               |
| `a`     | Analyze | AI 분석 → analysis.md      |
| `R`     | Review  | analysis.md 리뷰 및 피드백 |
| `p`     | Plan    | AI 구현 계획 → plan.md     |
| `c`     | Close   | status → closed            |
| `d`     | Discard | status → invalid           |
| `e`/`↵` | Edit    | $EDITOR로 brief.md 편집    |
| `f`     | Filter  | 필터 전환 (Active/All)     |
| `r`     | Refresh | 이슈 목록 새로고침         |
| `q`     | Quit    | 종료                       |

## Commands

```bash
# 의존성 설치
make deps

# 개발 실행
make run

# 상위 디렉토리 지정 실행
make run-path

# 빌드
make build

# 테스트
make test

# 바이너리 직접 실행
./dist/im --path /path/to/project
```

## AI Integration

Claude Code Headless mode를 통해 이슈 분석 및 구현 자동화.
