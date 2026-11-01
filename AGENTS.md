# AGENTS.md

ghq で管理しているリポジトリの Git worktree を決まった場所に作る Go の CLI。Claude Code の `WorktreeCreate` / `WorktreeRemove` フックとしても動く。
仕様は docs/spec.md、使い方は README.md にある。

## 構成

- `main.go`: カレントディレクトリとバージョンを渡して `internal/cli` を呼ぶ。
- `internal/cli`: サブコマンドの振り分けと、フックの入力（標準入力の JSON）の読み取り。
- `internal/layout`: worktree のパスを決める。ghq のルートと worktree のルートを調べる。
- `internal/worktree`: worktree の作成・削除・一覧と、作った直後の準備（`.worktreeinclude` のコピー、`ghq-worktree.setup` の実行）。
- `internal/claude`: `~/.claude/settings.json` のフックの書き換えと、スキル（`SKILL.md` を埋め込む）の書き出し。
- `internal/testenv`: テスト用に、一時ディレクトリへ ghq のルートとリモートの無いリポジトリを作る。

## コマンド

ツールのバージョンとタスクは `mise.toml` にまとめてある。

```sh
mise run test           # go test ./... と scripts/release_test.sh
mise run lint           # golangci-lint、jactionlint、shellcheck
mise run build          # バイナリを作る
mise run release-check  # GoReleaser の設定確認と試しビルド
mise run release-bump   # 標準入力の PR のラベルから、上げる桁を決める
mise run next-version   # 次のリリースのタグを出す（引数は patch、minor、major、skip）
```

CI（`.github/workflows/ci.yml`）は test、lint、release-check を別々のジョブで呼ぶ。
タスクを足したり変えたりしたら、CI のジョブもそろえる。

main にマージすると、`.github/workflows/release.yml` がタグを打ってリリースする。
上げる桁は PR のラベル（`release:major`、`release:minor`、なければパッチ）で決まり、`release:skip` の PR ではリリースしない。
PR を作ったら、変更の中身に合うラベルを付ける。ドキュメントや CI だけの PR には `release:skip` を付ける。

依存は Renovate（`renovate.json`）が上げる。メジャー以外は CI が通れば人が見ずにマージされ、パッチとしてリリースされる。
そのため、壊れたら `mise run test` か `mise run lint` が落ちるようにテストを保つ。
ツールを足すときは `mise.toml` に書けば、Renovate が拾う。

## 守ること

### 利用者の settings.json を壊さない

`install` / `uninstall` は、このコマンドのフックだけを足し引きする。ほかのキー、ほかのフック、キーの順番、書式は変えない。
Claude Code の書く形のファイルなら、install のあと uninstall すると 1 バイトも違わず元に戻ること（`TestEditHooksRoundTripKeepsFileAsIs`）。
シンボリックリンクの settings.json は、リンクを残してリンク先を書き換える（`TestInstallFollowsSymlink`）。

### 確認なしに作業を消さない

`remove` は、未コミットの変更がある worktree を `--force` なしでは消さない。
ブランチは、その worktree のために作ったもので、元の作業ディレクトリの HEAD にマージ済みのときだけ消す。`git branch -d` は upstream にマージ済みなら消してしまうので使わない。

### フックの標準出力はパスだけにする

`hook create` の標準出力は、Claude Code がそのまま worktree のパスとして読む。進捗や警告はすべて標準エラー出力に書く。
`ghq-worktree.setup` で走らせるコマンドの出力も、標準エラー出力に回す。

## 書き方

- CLI の出力、ヘルプ、スキル（`internal/claude/SKILL.md`）は英語で書く。外の人にも配るため。
- コード内のコメントは日本語で書く。コードを見れば分かることや、すぐ古くなる情報（日付や作業の番号など）は書かない。
