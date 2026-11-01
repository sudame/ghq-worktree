# ghq-worktree

ghq で管理しているリポジトリの Git worktree を、リポジトリの外の決まった場所に作る CLI です。
Claude Code の `WorktreeCreate` / `WorktreeRemove` フックとしても動きます。

```
~/worktrees/github.com/owner/repo/feat-x
└─ <worktree ルート>/<ghq ルートから見たリポジトリのパス>/<ブランチ名>
```

パスはリモート URL ではなく、ghq のルートの下のディレクトリ構成から決めます。そのため、リモートをまだ作っていないリポジトリでも使えます。

## 入れ方

```sh
mise use -g github:sudame/ghq-worktree
```

Claude Code で使うときは、続けて次を実行します。

```sh
ghq-worktree install claude-code
```

`~/.claude/settings.json` にフックを足し、`~/.claude/skills/ghq-worktree/SKILL.md` に AI 向けのスキルを置きます。`CLAUDE_CONFIG_DIR` を設定していれば、そちらに書きます。
以降、`claude --worktree`、`EnterWorktree` ツール、サブエージェントの `isolation: worktree` で作る worktree は、すべて上のパスにできます。リポジトリの中に `.claude/worktrees` はできません。

ghq-worktree を更新したら、`install claude-code` を実行し直してください。スキルが古いままだと、worktree を作るたびに警告が出ます。
外すときは `ghq-worktree uninstall claude-code` を実行します。

## 使い方

| コマンド | 動き |
|---|---|
| `ghq-worktree add <branch> [<start-point>]` | worktree を作り、パスを出力する。ブランチが無ければ `<start-point>`（省略時は HEAD）から作る |
| `ghq-worktree remove <branch\|path> [--force]` | worktree を消す。ブランチは、元の作業ディレクトリの HEAD にマージ済みのときだけ消す |
| `ghq-worktree list [--all]` | 今のリポジトリの worktree を出力する。`--all` は worktree ルートの下を全部出力する |
| `ghq-worktree path <branch>` | worktree のパスを出力する |

fzf で選んで移動する例です。

```sh
cd "$(ghq-worktree list --all | fzf)"
```

## 設定

設定は git config に書きます。

| キー | 既定値 | 意味 |
|---|---|---|
| `ghq-worktree.root` | `~/worktrees` | worktree を置くディレクトリ |
| `ghq-worktree.setup` | なし | worktree を作った直後に、その中で実行するコマンド。複数書ける。失敗しても警告だけ出して続ける |

```sh
git config --global --add ghq-worktree.setup "mise trust"
```

ghq のルートは `ghq root --all` で調べます。ghq が無ければ、環境変数 `GHQ_ROOT`、`git config ghq.root`、`~/ghq` の順に使います。

worktree を作るとき、リポジトリに `.worktreeinclude` があれば、そのパターンに合う gitignore 済みのファイル（`.env` など）を元のリポジトリからコピーします。Claude Code の既定の動きと同じです。

## 注意

- Claude Code のフックが作るブランチは `worktree-<name>` で、起点は今の HEAD です。Claude Code の既定（リモートのデフォルトブランチ）とは違います。
- サブエージェントの `isolation: worktree` で作った worktree は、Claude Code が消さないので残ります。`ghq-worktree remove` で消してください。
- Bash から直接 `git worktree add` を実行したときは、フックが走りません。
