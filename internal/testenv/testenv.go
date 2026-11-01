// Package testenv はテスト用に、ghq のルートとリモートの無いリポジトリを一時ディレクトリに作る。
package testenv

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sudame/ghq-worktree/internal/layout"
)

// Env はテスト用の環境。
type Env struct {
	Home         string
	GhqRoot      string
	WorktreeRoot string
	Repo         string
	GlobalConfig string
}

// New は HOME・ghq のルート・git の設定を一時ディレクトリに向け、example.com/me/repo を作る。
func New(t *testing.T) Env {
	t.Helper()
	tmp := layout.Canonical(t.TempDir())
	e := Env{
		Home:         filepath.Join(tmp, "home"),
		GhqRoot:      filepath.Join(tmp, "ghq"),
		WorktreeRoot: filepath.Join(tmp, "worktrees"),
		GlobalConfig: filepath.Join(tmp, "gitconfig"),
	}
	e.Repo = filepath.Join(e.GhqRoot, "example.com", "me", "repo")
	for _, d := range []string{e.Home, e.Repo} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", e.Home)
	t.Setenv("GHQ_ROOT", e.GhqRoot)
	t.Setenv("GIT_CONFIG_GLOBAL", e.GlobalConfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(e.Home, ".claude"))

	e.Git(t, "", "config", "--global", "user.name", "test")
	e.Git(t, "", "config", "--global", "user.email", "test@example.com")
	e.Git(t, "", "config", "--global", "init.defaultBranch", "main")
	e.Git(t, "", "config", "--global", "ghq-worktree.root", e.WorktreeRoot)
	e.Git(t, e.Repo, "init", "-q")
	e.Git(t, e.Repo, "commit", "-q", "--allow-empty", "-m", "init")
	return e
}

// Git は dir で git を実行し、失敗したらテストを止める。
func (e Env) Git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// WriteFile は dir/name に中身を書く。
func WriteFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
