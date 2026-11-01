package worktree

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sudame/ghq-worktree/internal/testenv"
)

func TestAddCreatesNewBranchWithoutRemote(t *testing.T) {
	e := testenv.New(t)
	var log bytes.Buffer
	path, err := Add(e.Repo, "feat/x", "", &log)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(e.WorktreeRoot, "example.com", "me", "repo", "feat-x")
	if path != want {
		t.Errorf("path = %s, want %s", path, want)
	}
	if got := strings.TrimSpace(e.Git(t, path, "branch", "--show-current")); got != "feat/x" {
		t.Errorf("branch = %s", got)
	}
}

func TestAddUsesExistingBranch(t *testing.T) {
	e := testenv.New(t)
	e.Git(t, e.Repo, "branch", "existing")
	path, err := Add(e.Repo, "existing", "", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(e.Git(t, path, "branch", "--show-current")); got != "existing" {
		t.Errorf("branch = %s", got)
	}
	if _, err := Add(e.Repo, "existing", "main", &bytes.Buffer{}); err == nil {
		t.Error("start point with existing branch should fail")
	}
}

func TestAddFromStartPoint(t *testing.T) {
	e := testenv.New(t)
	first := strings.TrimSpace(e.Git(t, e.Repo, "rev-parse", "HEAD"))
	e.Git(t, e.Repo, "commit", "-q", "--allow-empty", "-m", "second")
	path, err := Add(e.Repo, "old", first, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(e.Git(t, path, "rev-parse", "HEAD")); got != first {
		t.Errorf("HEAD = %s, want %s", got, first)
	}
}

func TestAddFromInsideWorktreeUsesMainRepoLayout(t *testing.T) {
	e := testenv.New(t)
	first, err := Add(e.Repo, "a", "", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Add(first, "b", "", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(e.WorktreeRoot, "example.com", "me", "repo", "b"); second != want {
		t.Errorf("path = %s, want %s", second, want)
	}
}

func TestAddFailsOutsideGhqRoot(t *testing.T) {
	e := testenv.New(t)
	other := filepath.Join(e.Home, "elsewhere")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	e.Git(t, other, "init", "-q")
	e.Git(t, other, "commit", "-q", "--allow-empty", "-m", "init")
	_, err := Add(other, "x", "", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not under any ghq root") {
		t.Errorf("err = %v", err)
	}
}

func TestAddCopiesWorktreeIncludeAndRunsSetup(t *testing.T) {
	e := testenv.New(t)
	testenv.WriteFile(t, e.Repo, ".gitignore", ".env\nnode_modules/\n")
	testenv.WriteFile(t, e.Repo, ".worktreeinclude", ".env\n")
	e.Git(t, e.Repo, "add", ".gitignore", ".worktreeinclude")
	e.Git(t, e.Repo, "commit", "-q", "-m", "ignore")
	testenv.WriteFile(t, e.Repo, ".env", "SECRET=1\n")
	testenv.WriteFile(t, e.Repo, "node_modules/x.js", "x\n")
	e.Git(t, "", "config", "--global", "--add", "ghq-worktree.setup", "touch setup-ran")
	e.Git(t, "", "config", "--global", "--add", "ghq-worktree.setup", "false")

	var log bytes.Buffer
	path, err := Add(e.Repo, "inc", "", &log)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(path, ".env")); err != nil || string(b) != "SECRET=1\n" {
		t.Errorf(".env not copied: %q %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(path, "node_modules")); !os.IsNotExist(err) {
		t.Errorf("node_modules should not be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "setup-ran")); err != nil {
		t.Errorf("setup command did not run: %v", err)
	}
	if !strings.Contains(log.String(), "warning: false") {
		t.Errorf("failing setup command not reported:\n%s", log.String())
	}
}

func TestRemoveDeletesMergedBranch(t *testing.T) {
	e := testenv.New(t)
	path, err := Add(e.Repo, "merged", "", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Remove(e.Repo, "merged", false, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("worktree still exists: %v", err)
	}
	if out := e.Git(t, e.Repo, "branch", "--list", "merged"); out != "" {
		t.Errorf("merged branch kept: %q", out)
	}
}

func TestRemoveKeepsUnmergedBranch(t *testing.T) {
	e := testenv.New(t)
	path, err := Add(e.Repo, "wip", "", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	e.Git(t, path, "commit", "-q", "--allow-empty", "-m", "wip")
	var log bytes.Buffer
	if err := Remove(e.Repo, path, false, &log); err != nil {
		t.Fatal(err)
	}
	if out := e.Git(t, e.Repo, "branch", "--list", "wip"); out == "" {
		t.Error("unmerged branch deleted")
	}
	if !strings.Contains(log.String(), "kept branch wip") {
		t.Errorf("log = %s", log.String())
	}
}

func TestRemoveDirtyNeedsForce(t *testing.T) {
	e := testenv.New(t)
	path, err := Add(e.Repo, "dirty", "", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	testenv.WriteFile(t, path, "new.txt", "x")
	if err := Remove(e.Repo, "dirty", false, &bytes.Buffer{}); err == nil {
		t.Fatal("removing a dirty worktree without --force should fail")
	}
	if err := Remove(e.Repo, "dirty", true, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveRefusesMainWorktree(t *testing.T) {
	e := testenv.New(t)
	if err := Remove(e.Repo, e.Repo, true, &bytes.Buffer{}); err == nil {
		t.Error("removing the main worktree should fail")
	}
}

func TestListAndListAll(t *testing.T) {
	e := testenv.New(t)
	a, err := Add(e.Repo, "a", "", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := List(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Path != e.Repo || entries[1].Path != a || entries[1].Branch != "a" {
		t.Errorf("List = %+v", entries)
	}
	all, err := ListAll(e.WorktreeRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0] != a {
		t.Errorf("ListAll = %v", all)
	}
	none, err := ListAll(filepath.Join(e.Home, "missing"))
	if err != nil || len(none) != 0 {
		t.Errorf("ListAll on missing root = %v, %v", none, err)
	}
}

func TestRemoveKeepsBranchMergedOnlyIntoUpstream(t *testing.T) {
	e := testenv.New(t)
	remote := filepath.Join(e.Home, "remote.git")
	e.Git(t, "", "init", "-q", "--bare", remote)
	e.Git(t, e.Repo, "remote", "add", "origin", remote)
	e.Git(t, e.Repo, "push", "-q", "origin", "main")
	path, err := Add(e.Repo, "pushed", "", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	e.Git(t, path, "commit", "-q", "--allow-empty", "-m", "work")
	e.Git(t, path, "push", "-q", "-u", "origin", "pushed")
	if err := Remove(e.Repo, "pushed", false, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if out := e.Git(t, e.Repo, "branch", "--list", "pushed"); out == "" {
		t.Error("branch not merged into HEAD was deleted")
	}
}

func TestRemoveKeepsBranchSwitchedToInsideWorktree(t *testing.T) {
	e := testenv.New(t)
	e.Git(t, e.Repo, "branch", "develop")
	path, err := Add(e.Repo, "made", "", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	e.Git(t, path, "switch", "-q", "develop")
	if err := Remove(e.Repo, path, false, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if out := e.Git(t, e.Repo, "branch", "--list", "develop"); out == "" {
		t.Error("develop was deleted")
	}
}

func TestRemoveByBranchChecksBranchName(t *testing.T) {
	e := testenv.New(t)
	if _, err := Add(e.Repo, "feat/x", "", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := Remove(e.Repo, "feat-x", true, &bytes.Buffer{}); err == nil {
		t.Error("remove feat-x removed the worktree of feat/x")
	}
}

func TestExisting(t *testing.T) {
	e := testenv.New(t)
	if _, ok := Existing(e.Repo, "w"); ok {
		t.Fatal("Existing before Add")
	}
	path, err := Add(e.Repo, "w", "", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := Existing(e.Repo, "w"); !ok || got != path {
		t.Errorf("Existing = %q, %v", got, ok)
	}
}
