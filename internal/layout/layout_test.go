package layout

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSanitizeBranch(t *testing.T) {
	for in, want := range map[string]string{
		"main":         "main",
		"feat/x":       "feat-x",
		"a/b:c":        "a-b-c",
		"worktree-foo": "worktree-foo",
	} {
		if got := SanitizeBranch(in); got != want {
			t.Errorf("SanitizeBranch(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRelToGhq(t *testing.T) {
	roots := []string{"/a/ghq", "/b/src"}
	for repo, want := range map[string]string{
		"/a/ghq/github.com/me/x": "github.com/me/x",
		"/b/src/local":           "local",
	} {
		got, err := RelToGhq(filepath.FromSlash(repo), roots)
		if err != nil || got != filepath.FromSlash(want) {
			t.Errorf("RelToGhq(%s) = %q, %v; want %q", repo, got, err, want)
		}
	}
	for _, repo := range []string{"/a/ghq", "/a/ghqx/y", "/c/z", "/a"} {
		if got, err := RelToGhq(repo, roots); err == nil {
			t.Errorf("RelToGhq(%s) = %q, want error", repo, got)
		}
	}
}

func TestGhqRootsWithoutGhqCommand(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip(err)
	}
	bin := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	home := Canonical(t.TempDir())
	t.Setenv("PATH", bin)
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	t.Setenv("GHQ_ROOT", "")
	roots, err := GhqRoots()
	if err != nil || len(roots) != 1 || roots[0] != filepath.Join(home, "ghq") {
		t.Errorf("default: %v, %v", roots, err)
	}

	cmd := exec.Command("git", "config", "--global", "ghq.root", "~/src")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	roots, err = GhqRoots()
	if err != nil || len(roots) != 1 || roots[0] != filepath.Join(home, "src") {
		t.Errorf("git config: %v, %v", roots, err)
	}

	t.Setenv("GHQ_ROOT", "/x"+string(filepath.ListSeparator)+"/y")
	roots, err = GhqRoots()
	if err != nil || len(roots) != 2 || roots[0] != "/x" || roots[1] != "/y" {
		t.Errorf("GHQ_ROOT: %v, %v", roots, err)
	}
}
