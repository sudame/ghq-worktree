package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sudame/ghq-worktree/internal/testenv"
)

func run(t *testing.T, dir, stdin string, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := Run(args, Env{Version: "v0.1.0", Dir: dir, Stdin: strings.NewReader(stdin), Stdout: &stdout, Stderr: &stderr})
	return stdout.String(), stderr.String(), err
}

func TestHookCreatePrintsOnlyPath(t *testing.T) {
	e := testenv.New(t)
	in := `{"session_id":"s","cwd":"` + e.Repo + `","hook_event_name":"WorktreeCreate","name":"bold-oak"}`
	stdout, _, err := run(t, e.Home, in, "hook", "create")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(e.WorktreeRoot, "example.com", "me", "repo", "worktree-bold-oak") + "\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestHookRemove(t *testing.T) {
	e := testenv.New(t)
	in := `{"cwd":"` + e.Repo + `","name":"tmp"}`
	stdout, _, err := run(t, e.Home, in, "hook", "create")
	if err != nil {
		t.Fatal(err)
	}
	path := strings.TrimSpace(stdout)
	testenv.WriteFile(t, path, "dirty.txt", "x")

	if _, _, err := run(t, e.Home, `{"worktree_path":"`+path+`"}`, "hook", "remove"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("worktree still exists: %v", err)
	}
	// 消えたあとにもう一度呼ばれても失敗しない
	if _, _, err := run(t, e.Home, `{"worktree_path":"`+path+`"}`, "hook", "remove"); err != nil {
		t.Errorf("second remove: %v", err)
	}
}

func TestHookCreateWarnsOnStaleSkill(t *testing.T) {
	e := testenv.New(t)
	if _, _, err := run(t, e.Home, "", "install", "claude-code"); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	in := `{"cwd":"` + e.Repo + `","name":"n"}`
	err := Run([]string{"hook", "create"}, Env{Version: "v0.2.0", Dir: e.Home, Stdin: strings.NewReader(in), Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "installed skill is from ghq-worktree v0.1.0") {
		t.Errorf("stderr = %s", stderr.String())
	}
}

func TestAddPathListRemove(t *testing.T) {
	e := testenv.New(t)
	stdout, _, err := run(t, e.Repo, "", "add", "feat/y")
	if err != nil {
		t.Fatal(err)
	}
	path := strings.TrimSpace(stdout)

	got, _, err := run(t, e.Repo, "", "path", "feat/y")
	if err != nil || strings.TrimSpace(got) != path {
		t.Errorf("path = %q, %v; want %q", got, err, path)
	}
	list, _, err := run(t, e.Repo, "", "list")
	if err != nil || list != e.Repo+"\n"+path+"\n" {
		t.Errorf("list = %q, %v", list, err)
	}
	all, _, err := run(t, e.Home, "", "list", "--all")
	if err != nil || all != path+"\n" {
		t.Errorf("list --all = %q, %v", all, err)
	}
	if _, _, err := run(t, e.Repo, "", "remove", "feat/y", "--force"); err != nil {
		t.Fatal(err)
	}
}

func TestBadArguments(t *testing.T) {
	e := testenv.New(t)
	for _, args := range [][]string{{}, {"nope"}, {"add"}, {"remove", "x", "--bogus"}, {"install", "vim"}, {"hook", "x"}} {
		if _, _, err := run(t, e.Repo, "", args...); err == nil {
			t.Errorf("%v succeeded, want error", args)
		}
	}
}

func TestHookCreateReusesExistingWorktree(t *testing.T) {
	e := testenv.New(t)
	in := `{"cwd":"` + e.Repo + `","name":"again"}`
	first, _, err := run(t, e.Home, in, "hook", "create")
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := run(t, e.Home, in, "hook", "create")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("second create = %q, want %q", second, first)
	}
}
