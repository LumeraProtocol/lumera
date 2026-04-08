# BRIDGE Hooks for Claude Code

Hooks run shell commands, LLM prompts, or agent checks at specific points
in Claude Code's lifecycle. They're configured in `.claude/settings.json`.

## Configured Hooks

### SessionStart: Validate BRIDGE context on startup

Runs `session-start.sh` when a new session begins. Checks whether
`docs/context.json` exists and reports its staleness so Claude can
suggest a context sync if needed.

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          {
            "type": "command",
            "command": "\"$CLAUDE_PROJECT_DIR/.claude/hooks/session-start.sh\"",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

### PreToolUse: Auto-approve safe `cd && git ...` checks

Runs `auto-approve-cd-git.sh` before Bash tool calls. It approves
read-only git inspection commands that are already scoped to a target
directory, and leaves everything else to Claude's normal permission flow.

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "\"$CLAUDE_PROJECT_DIR/.claude/hooks/auto-approve-cd-git.sh\"",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```

### PostToolUse: Auto-lint after file edits

Runs `post-edit-lint.sh` after Claude edits or writes files. Reads the
lint command from `docs/context.json` → `commands_to_run.lint` and falls
back to common defaults.

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "\"$CLAUDE_PROJECT_DIR/.claude/hooks/post-edit-lint.sh\"",
            "timeout": 30
          }
        ]
      }
    ]
  }
}
```

## Available Hook Events

| Event | When it fires | Matcher input |
|-------|--------------|---------------|
| `SessionStart` | Session begins or resumes | `startup`, `resume`, `clear`, `compact` |
| `UserPromptSubmit` | User submits a prompt | (no matcher) |
| `PreToolUse` | Before a tool call — can block it | Tool name |
| `PermissionRequest` | Permission dialog appears | Tool name |
| `PostToolUse` | After a tool call succeeds | Tool name |
| `PostToolUseFailure` | After a tool call fails | Tool name |
| `Notification` | Notification sent | Notification type |
| `SubagentStart` | Subagent spawned | Agent type name |
| `SubagentStop` | Subagent completes | Agent type name |
| `Stop` | Claude finishes responding | (no matcher) |
| `TaskCompleted` | Task marked complete | (no matcher) |
| `ConfigChange` | Config file changes | Config source |
| `PreCompact` | Before context compaction | `manual`, `auto` |
| `SessionEnd` | Session terminates | End reason |

## Hook Types

1. **Command** (`type: "command"`) — Run a shell script. Receives JSON on stdin.
2. **Prompt** (`type: "prompt"`) — Single LLM call for yes/no decisions.
3. **Agent** (`type: "agent"`) — Multi-turn subagent with tool access for verification.

## Customizing for Your Project

After running `/bridge-requirements` or `/bridge-requirements-only`,
update hook scripts with your project's lint/test commands from
`docs/context.json` → `commands_to_run`.

## Adding Custom Hooks

Add hooks to `.claude/settings.json` (shared) or `.claude/settings.local.json`
(personal). Use `$CLAUDE_PROJECT_DIR` to reference scripts relative to the
project root. See https://code.claude.com/docs/en/hooks for full reference.
