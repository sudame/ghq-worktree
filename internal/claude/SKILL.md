---
name: ghq-worktree
description: Use whenever you create, enter, list, or remove a git worktree, isolate work on a branch, or run work in parallel. Explains where worktrees live on this machine and which commands to use instead of git worktree / gwq.
---
{{HEADER}}

# Git worktrees with ghq-worktree

On this machine, Claude Code creates and removes worktrees through `ghq-worktree`, which is registered as the `WorktreeCreate` / `WorktreeRemove` hook.
Worktrees live outside the repository, at

```
<worktree root>/<repository path under the ghq root>/<branch, with / and : replaced by ->
e.g. ~/worktrees/github.com/owner/repo/feat-x
```

so nothing needs to be added to `.gitignore`. Repositories without a remote work too.

## Creating a worktree

- To isolate work quickly, use the `EnterWorktree` tool (or `isolation: worktree` for a subagent). The hook creates branch `worktree-<name>` from the current HEAD.
- To work on a branch with a specific name, or on an existing branch, run `ghq-worktree add <branch> [<start-point>]`. It prints only the new path on stdout; `cd` there in a separate command.
- Do not run `git worktree add` from Bash. The hook does not run for it, so the worktree ends up in the wrong place and setup is skipped.

## Finding a worktree

- `ghq-worktree list` prints every worktree of the current repository (including the main one).
- `ghq-worktree list --all` prints every worktree under the worktree root.
- `ghq-worktree path <branch>` prints where the worktree for `<branch>` is (or would be).

## Removing a worktree

- `ghq-worktree remove <branch|path>` removes the worktree. Add `--force` to discard uncommitted changes.
- The branch is deleted only if it is fully merged; otherwise it is kept and a message is printed.
- Worktrees created by subagents with `isolation: worktree` are not removed automatically. Remove them with `ghq-worktree remove` when they are no longer needed.
