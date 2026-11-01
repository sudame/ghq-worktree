# ghq-worktree 仕様

## 目的

ghq で管理しているリポジトリの worktree を、決まった場所に作る。
リモートが無いリポジトリでも動くこと、Claude Code の worktree 機能と同じ場所を使うことを目指す。

gwq との違いは次の 3 点。

- worktree のパスをリモート URL からではなく、ghq のルートから見たリポジトリのパスで決める。
- Claude Code の `WorktreeCreate` / `WorktreeRemove` フックとして動く。
- 引数を少なくし、AI が迷わないようにする。AI には原則として Claude Code の `EnterWorktree` を使わせ、このコマンドは直接叩かせない。

## worktree のパス

```
<worktree ルート>/<ghq ルートから見たリポジトリのパス>/<ブランチ名>
例: ~/worktrees/github.com/sudame/hoge/feat-x
```

- ブランチ名の `/` と `:` は `-` に置き換える。gwq の既定と同じなので、gwq で作った既存の worktree と同じ場所になる。
- worktree ルートは `git config ghq-worktree.root` で決める。未設定なら `~/worktrees`。
- ghq ルートは `ghq root --all` で得る。ghq が無ければ `git config ghq.root`、それも無ければ `~/ghq`。
- リポジトリがどの ghq ルートの下にも無いときはエラーにする。
- worktree の中で実行したときは、元のリポジトリ（`git rev-parse --git-common-dir` の親）を基準にする。

## コマンド

| コマンド | 動き |
|---|---|
| `ghq-worktree add <branch> [<start-point>]` | ブランチがあればそれを、無ければ `<start-point>`（省略時は HEAD）から新しく作って worktree にする。作ったパスを標準出力に出す |
| `ghq-worktree remove <branch\|path> [--force]` | worktree を消す。ブランチは、元の作業ディレクトリの HEAD にマージ済みなら消す。未マージなら残して警告を出す |
| `ghq-worktree list [--all]` | 今のリポジトリの worktree のパスを出す。`--all` は worktree ルートの下を全部出す |
| `ghq-worktree path <branch>` | パスを出すだけ。`cd "$(ghq-worktree path feat-x)"` のように使う |
| `ghq-worktree hook create` | Claude Code の `WorktreeCreate` 用。後述 |
| `ghq-worktree hook remove` | Claude Code の `WorktreeRemove` 用。後述 |

`remove` がブランチを消すのは、次の 2 つを満たすときだけ。

- そのブランチのために作った worktree であること。worktree のディレクトリ名とブランチ名（置き換え後）が一致するかで見る。worktree の中で別のブランチに切り替えていたら、そのブランチは消さない。
- 元の作業ディレクトリの HEAD にマージ済みであること。`git branch -d` は upstream にマージ済みのブランチも消すので、使わずに `git merge-base --is-ancestor` で確かめる。

`remove <branch>` は、ブランチ名から計算したパスの worktree が、そのブランチをチェックアウトしているときだけ対象にする。`feat/x` と `feat-x` は同じパスになるためである。

`list --all` を用意する理由: gwq の `gwq list -g` は、リモートの無いリポジトリの worktree を一覧に出さない（実際に確かめた）。人間が fzf などで選んで移動できるよう、こちらで一覧を出す。

### 作った直後の準備

`add` と `hook create` は、worktree を作ったあとに次を行う。

1. `.worktreeinclude` に合う gitignore 済みのファイル（`.env` など）を元のリポジトリからコピーする。Claude Code の既定の挙動と同じ。フックを使うと Claude Code はこれをしなくなるので、こちらで肩代わりする。
2. `git config --get-all ghq-worktree.setup` に並んだコマンドを、worktree の中で順に実行する。失敗しても警告だけ出して続ける。自分の環境では `mise trust` をここに入れ、gwq の設定にリポジトリごとに並べていた `setup_commands` をやめる。

## Claude Code のフック

### `hook create`

- 標準入力の JSON から `cwd` と `name` を読む。
- ブランチ名は `worktree-<name>`（Claude Code の既定と同じ）。
- 起点は `cwd` の HEAD。Claude Code の既定（リモートのデフォルトブランチ）とは違うが、リモートが無くても動くことを優先する。
- 同じブランチの worktree がすでにあれば、作らずにそのパスを返す。Claude Code の既定でも、同じ名前を渡すと残してあった worktree を開き直すため。
- 作った worktree の絶対パスだけを標準出力に出す。ログは標準エラー出力に出す。

### `hook remove`

- 標準入力の JSON から `worktree_path` を読み、`remove --force` と同じことをする。
- Claude Code はこのフックが成功すれば削除できたとみなすので、ここで実際に消す。

## 配布

Go の単体バイナリを GoReleaser で作り、GitHub Releases に置く。mise なら `github:sudame/ghq-worktree` で入る。配るのはこのバイナリだけで、Claude Code のプラグインは作らない。

Claude Code への組み込みは、バイナリ自身が設定を書き換えて行う。

| コマンド | 動き |
|---|---|
| `ghq-worktree install claude-code` | 下の 2 つを書き込む。何度実行しても同じ結果になる |
| `ghq-worktree uninstall claude-code` | 書き込んだものだけを消す |

書き込むものは次の 2 つ。

1. `~/.claude/settings.json` の `hooks` に、`WorktreeCreate` / `WorktreeRemove` として `ghq-worktree hook create` / `hook remove` を足す。ほかのキーとフックには触らない。キーの順番と 2 文字のインデントは保つ。
2. `~/.claude/skills/ghq-worktree/SKILL.md` に AI 向けのスキルを置く。worktree は `EnterWorktree` で作る、Bash から `git worktree add` を叩かない（フックが走らない）、と書く。先頭に「生成したバージョン」と「直接編集しない」旨のコメントを入れる（yomiyasu-go と同じ形）。

プラグインにしない理由は次の 3 つ。

- バイナリとフックの定義が別々に更新されてずれることがない。
- プラグインのフックは `claude --worktree --tmux` のときに走らないが、settings.json のフックは走る。
- 入れるものがバイナリ 1 つで済む。

`install` は既存の settings.json を読んでから書き換える。JSON として読めなければ何も書かずに失敗する。

- 元のファイルが Claude Code の書く形（2 文字インデントで、配列も展開した形）なら、書き換えたあとも同じ形に整える。それ以外の書式なら整形せず、変えた部分のほかは元のバイト列のまま残す。
- 同じフックのグループに利用者のコマンドが並んでいても、消すのはこのコマンドのフックだけにする。
- ほかの `WorktreeCreate` / `WorktreeRemove` フックがすでにあれば、警告を出す（どちらも worktree を作ろうとして食い違うおそれがあるため）。

## 既知の制約

- バイナリを上げてスキルの中身が変わったときは、`install claude-code` を実行し直す必要がある。古いままなら `hook create` が標準エラー出力に警告を出す。
- サブエージェントの `isolation: worktree` で作った worktree は、終わっても `WorktreeRemove` が呼ばれず残る。`ghq-worktree remove` で消す。

## このリポジトリの外でやること

- sudame-claude-plugins の gwq プラグインを外す。
- CLAUDE.md の「Git は worktree ベースで作業する」を、このプラグインを前提に書き換える。
- gwq の設定から `repository_settings` を消す。gwq は人間の移動用に残す。
