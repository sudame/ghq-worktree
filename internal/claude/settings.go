// Package claude は Claude Code への組み込み（フック・スキル）を扱う。
package claude

import (
	"bytes"
	"errors"
	"fmt"
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
func EditHooks(data []byte, add bool) ([]byte, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
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

	var err error
	for _, h := range hookEvents {
		path := "hooks." + h.event
		cur := gjson.GetBytes(data, path)
		if cur.Exists() && !cur.IsArray() {
			return nil, fmt.Errorf("%q in settings.json is not an array", path)
		}
		var items []string
		for _, it := range cur.Array() {
			if !isOurs(it) {
				items = append(items, it.Raw)
			}
		}
		if add {
			items = append(items, fmt.Sprintf(`{"hooks":[{"type":"command","command":%q}]}`, h.command))
		}
		if len(items) == 0 {
			if cur.Exists() {
				if data, err = sjson.DeleteBytes(data, path); err != nil {
					return nil, err
				}
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
	return pretty.PrettyOptions(data, &pretty.Options{Indent: "  ", SortKeys: false}), nil
}

func isOurs(item gjson.Result) bool {
	for _, h := range item.Get("hooks").Array() {
		if strings.HasPrefix(h.Get("command").String(), hookPrefix) {
			return true
		}
	}
	return false
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

func updateSettings(dir string, add bool) error {
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
	if bytes.Equal(out, data) {
		return nil
	}
	return writeFile(path, out)
}
