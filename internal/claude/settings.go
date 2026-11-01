// Package claude は Claude Code への組み込み（フック・スキル）を扱う。
package claude

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
)

const hookPrefix = "ghq-worktree hook "

var hookEvents = []struct{ event, command string }{
	{"WorktreeCreate", hookPrefix + "create"},
	{"WorktreeRemove", hookPrefix + "remove"},
}

// ConfigDir は Claude Code の設定ディレクトリを返す。
func ConfigDir() (string, error) {
	if v := os.Getenv("CLAUDE_CONFIG_DIR"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// EditHooks は settings.json の中身に、このコマンドのフックを足す（add が false なら消す）。
// ほかのキーとフックには触らず、キーの順番を保つ。
// 元が Claude Code の書く 2 文字インデントの形なら、同じ形に整えて返す。そうでなければ整形しない。
func EditHooks(data []byte, add bool) ([]byte, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}\n")
	}
	if !gjson.ValidBytes(data) {
		return nil, errors.New("settings.json is not valid JSON")
	}
	if !gjson.ParseBytes(data).IsObject() {
		return nil, errors.New("settings.json is not a JSON object")
	}
	if h := gjson.GetBytes(data, "hooks"); h.Exists() && !h.IsObject() {
		return nil, errors.New(`"hooks" in settings.json is not an object`)
	}
	trailingNewline := bytes.HasSuffix(data, []byte("\n"))
	canonical := bytes.Equal(bytes.TrimRight(prettyJSON(data), "\n"), bytes.TrimRight(data, "\n"))

	var err error
	for _, h := range hookEvents {
		path := "hooks." + h.event
		cur := gjson.GetBytes(data, path)
		if cur.Exists() && !cur.IsArray() {
			return nil, fmt.Errorf("%q in settings.json is not an array", path)
		}
		changed := add
		var items []string
		for _, group := range cur.Array() {
			raw, keep, err := withoutOurs(group)
			if err != nil {
				return nil, err
			}
			if raw != group.Raw || !keep {
				changed = true
			}
			if keep {
				items = append(items, raw)
			}
		}
		if !changed {
			continue
		}
		if add {
			items = append(items, fmt.Sprintf(`{"hooks":[{"type":"command","command":%q}]}`, h.command))
		}
		if len(items) == 0 {
			if data, err = sjson.DeleteBytes(data, path); err != nil {
				return nil, err
			}
			continue
		}
		if data, err = sjson.SetRawBytes(data, path, []byte("["+strings.Join(items, ",")+"]")); err != nil {
			return nil, err
		}
	}

	if h := gjson.GetBytes(data, "hooks"); h.Exists() && len(h.Map()) == 0 {
		if data, err = sjson.DeleteBytes(data, "hooks"); err != nil {
			return nil, err
		}
	}
	if canonical {
		data = prettyJSON(data)
	}
	data = bytes.TrimRight(data, "\n")
	if trailingNewline {
		data = append(data, '\n')
	}
	return data, nil
}

func prettyJSON(data []byte) []byte {
	return pretty.PrettyOptions(data, &pretty.Options{Indent: "  ", SortKeys: false})
}

func isOurs(hook gjson.Result) bool {
	return strings.HasPrefix(hook.Get("command").String(), hookPrefix)
}

// withoutOurs はフックのグループから、このコマンドのフックだけを除く。
// グループが空になったら keep を false にする。
func withoutOurs(group gjson.Result) (raw string, keep bool, err error) {
	var rest []string
	found := false
	for _, h := range group.Get("hooks").Array() {
		if isOurs(h) {
			found = true
			continue
		}
		rest = append(rest, h.Raw)
	}
	if !found {
		return group.Raw, true, nil
	}
	if len(rest) == 0 {
		return "", false, nil
	}
	raw, err = sjson.SetRaw(group.Raw, "hooks", "["+strings.Join(rest, ",")+"]")
	return raw, err == nil, err
}

// ForeignWorktreeHooks は、このコマンド以外の WorktreeCreate / WorktreeRemove フックのコマンドを返す。
func ForeignWorktreeHooks(data []byte) []string {
	var out []string
	for _, h := range hookEvents {
		for _, group := range gjson.GetBytes(data, "hooks."+h.event).Array() {
			for _, hook := range group.Get("hooks").Array() {
				if !isOurs(hook) {
					out = append(out, h.event+": "+hook.Get("command").String())
				}
			}
		}
	}
	return out
}

// writeFile は path がシンボリックリンクならリンク先を、一時ファイル経由で書き換える。
func writeFile(path string, data []byte) error {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func updateSettings(dir string, add bool, log io.Writer) error {
	path := filepath.Join(dir, "settings.json")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err != nil && !add {
		return nil
	}
	out, err := EditHooks(data, add)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if add {
		for _, c := range ForeignWorktreeHooks(out) {
			_, _ = fmt.Fprintf(log, "warning: %s also has %s; it may conflict with ghq-worktree\n", path, c)
		}
	}
	if bytes.Equal(out, data) {
		return nil
	}
	return writeFile(path, out)
}
