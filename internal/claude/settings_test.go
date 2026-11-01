package claude

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func readFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/settings.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestEditHooksAddsBothHooks(t *testing.T) {
	out, err := EditHooks(readFixture(t), true)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hookEvents {
		got := gjson.GetBytes(out, "hooks."+h.event+".0.hooks.0.command").String()
		if got != h.command {
			t.Errorf("%s command = %q, want %q", h.event, got, h.command)
		}
	}
	if got := gjson.GetBytes(out, "hooks.PreToolUse.0.hooks.0.command").String(); got != "/home/user/bin/bash-guard" {
		t.Errorf("existing hook changed: %q", got)
	}
}

func TestEditHooksRoundTripKeepsFileAsIs(t *testing.T) {
	orig := readFixture(t)
	added, err := EditHooks(orig, true)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := EditHooks(added, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(removed) != string(orig) {
		t.Errorf("install then uninstall changed the file:\n%s", removed)
	}
}

func TestEditHooksOnlyAddsHooksToFile(t *testing.T) {
	orig := readFixture(t)
	added, err := EditHooks(orig, true)
	if err != nil {
		t.Fatal(err)
	}
	// 足した行を除けば元の行がそのまま同じ順で残っていること
	origLines := strings.Split(string(orig), "\n")
	i := 0
	for line := range strings.SplitSeq(string(added), "\n") {
		if i < len(origLines) && line == origLines[i] {
			i++
		}
	}
	if i != len(origLines) {
		t.Errorf("line %d of the original (%q) was not kept:\n%s", i+1, origLines[i], added)
	}
}

func TestEditHooksIsIdempotent(t *testing.T) {
	once, err := EditHooks(readFixture(t), true)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := EditHooks(once, true)
	if err != nil {
		t.Fatal(err)
	}
	if string(once) != string(twice) {
		t.Errorf("second install changed the file:\n%s", twice)
	}
}

func TestEditHooksKeepsOtherWorktreeHooks(t *testing.T) {
	in := `{"hooks":{"WorktreeCreate":[{"hooks":[{"type":"command","command":"other"}]}]}}`
	added, err := EditHooks([]byte(in), true)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(gjson.GetBytes(added, "hooks.WorktreeCreate").Array()); n != 2 {
		t.Fatalf("WorktreeCreate has %d entries, want 2", n)
	}
	removed, err := EditHooks(added, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := gjson.GetBytes(removed, "hooks.WorktreeCreate.0.hooks.0.command").String(); got != "other" {
		t.Errorf("other hook lost: %s", removed)
	}
	if gjson.GetBytes(removed, "hooks.WorktreeRemove").Exists() {
		t.Errorf("empty WorktreeRemove left behind: %s", removed)
	}
}

func TestEditHooksEmptyInput(t *testing.T) {
	out, err := EditHooks(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !gjson.GetBytes(out, "hooks.WorktreeCreate").Exists() {
		t.Errorf("hooks not added: %s", out)
	}
}

func TestEditHooksRejectsBrokenInput(t *testing.T) {
	for _, in := range []string{`{"a":`, `[]`, `{"hooks":[]}`, `{"hooks":{"WorktreeCreate":{}}}`} {
		if _, err := EditHooks([]byte(in), true); err == nil {
			t.Errorf("EditHooks(%s) succeeded, want error", in)
		}
	}
}

func TestInstallAndUninstall(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	orig := readFixture(t)
	if err := os.WriteFile(settings, orig, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Install(dir, "v1.2.3", io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := InstalledSkillVersion(dir); got != "v1.2.3" {
		t.Errorf("InstalledSkillVersion = %q", got)
	}
	fi, err := os.Stat(settings)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
	}

	if err := Uninstall(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(orig) {
		t.Errorf("settings.json not restored:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", "ghq-worktree")); !os.IsNotExist(err) {
		t.Errorf("skill directory left behind: %v", err)
	}
}

func TestInstallFollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles-settings.json")
	if err := os.WriteFile(real, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(dir, "conf")
	if err := os.Mkdir(conf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(conf, "settings.json")); err != nil {
		t.Fatal(err)
	}
	if err := Install(conf, "dev", io.Discard); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(filepath.Join(conf, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("symlink was replaced by a regular file")
	}
	data, err := os.ReadFile(real)
	if err != nil {
		t.Fatal(err)
	}
	if !gjson.GetBytes(data, "hooks.WorktreeCreate").Exists() {
		t.Errorf("link target not updated: %s", data)
	}
}

func TestEditHooksKeepsNonStandardFormatting(t *testing.T) {
	in := "{\n    \"permissions\": {\"deny\": [\"a\"]}\n}"
	added, err := EditHooks([]byte(in), true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(added), "{\n    \"permissions\": {\"deny\": [\"a\"]}") {
		t.Errorf("existing formatting changed:\n%s", added)
	}
	removed, err := EditHooks(added, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(removed) != in {
		t.Errorf("round trip = %q, want %q", removed, in)
	}
}

func TestEditHooksKeepsUserHookInSameGroup(t *testing.T) {
	in := `{"hooks":{"WorktreeRemove":[{"hooks":[{"type":"command","command":"ghq-worktree hook remove"},{"type":"command","command":"notify.sh"}]}]}}`
	removed, err := EditHooks([]byte(in), false)
	if err != nil {
		t.Fatal(err)
	}
	hooks := gjson.GetBytes(removed, "hooks.WorktreeRemove.0.hooks").Array()
	if len(hooks) != 1 || hooks[0].Get("command").String() != "notify.sh" {
		t.Errorf("user hook lost: %s", removed)
	}
}

func TestForeignWorktreeHooks(t *testing.T) {
	in := `{"hooks":{"WorktreeCreate":[{"hooks":[{"type":"command","command":"other"}]},{"hooks":[{"type":"command","command":"ghq-worktree hook create"}]}]}}`
	got := ForeignWorktreeHooks([]byte(in))
	if len(got) != 1 || got[0] != "WorktreeCreate: other" {
		t.Errorf("ForeignWorktreeHooks = %v", got)
	}
}
