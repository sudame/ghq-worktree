// Package cli はサブコマンドを振り分ける。
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sudame/ghq-worktree/internal/claude"
	"github.com/sudame/ghq-worktree/internal/layout"
	"github.com/sudame/ghq-worktree/internal/worktree"
)

// Env はコマンドの入出力と、カレントディレクトリ・バージョンをまとめる。
type Env struct {
	Version string
	Dir     string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

const usage = `usage:
  ghq-worktree add <branch> [<start-point>]
  ghq-worktree remove <branch|path> [--force]
  ghq-worktree list [--all]
  ghq-worktree path <branch>
  ghq-worktree install claude-code
  ghq-worktree uninstall claude-code
  ghq-worktree hook create|remove    (called by Claude Code)
  ghq-worktree version
`

var errUsage = errors.New("invalid arguments")

// Run は args を解釈して実行する。
func Run(args []string, env Env) error {
	if len(args) == 0 {
		_, _ = io.WriteString(env.Stderr, usage)
		return errUsage
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "add":
		pos, _, err := parse(rest, nil)
		if err != nil || len(pos) < 1 || len(pos) > 2 {
			return usageErr(env, err)
		}
		start := ""
		if len(pos) == 2 {
			start = pos[1]
		}
		path, err := worktree.Add(env.Dir, pos[0], start, env.Stderr)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(env.Stdout, path)
		return err

	case "remove":
		pos, flags, err := parse(rest, map[string]string{"--force": "force", "-f": "force"})
		if err != nil || len(pos) != 1 {
			return usageErr(env, err)
		}
		return worktree.Remove(env.Dir, pos[0], flags["force"], env.Stderr)

	case "list":
		pos, flags, err := parse(rest, map[string]string{"--all": "all", "-a": "all"})
		if err != nil || len(pos) != 0 {
			return usageErr(env, err)
		}
		return list(env, flags["all"])

	case "path":
		pos, _, err := parse(rest, nil)
		if err != nil || len(pos) != 1 {
			return usageErr(env, err)
		}
		repo, err := layout.RepoRoot(env.Dir)
		if err != nil {
			return err
		}
		path, err := layout.WorktreePath(repo, pos[0])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(env.Stdout, path)
		return err

	case "install", "uninstall":
		if len(rest) != 1 || rest[0] != "claude-code" {
			return usageErr(env, nil)
		}
		dir, err := claude.ConfigDir()
		if err != nil {
			return err
		}
		if cmd == "install" {
			if err := claude.Install(dir, env.Version); err != nil {
				return err
			}
			_, err = fmt.Fprintf(env.Stderr, "installed hooks and skill into %s\n", dir)
			return err
		}
		if err := claude.Uninstall(dir); err != nil {
			return err
		}
		_, err = fmt.Fprintf(env.Stderr, "removed hooks and skill from %s\n", dir)
		return err

	case "hook":
		if len(rest) != 1 {
			return usageErr(env, nil)
		}
		switch rest[0] {
		case "create":
			return hookCreate(env)
		case "remove":
			return hookRemove(env)
		}
		return usageErr(env, nil)

	case "version", "--version", "-v":
		_, err := fmt.Fprintln(env.Stdout, env.Version)
		return err

	case "help", "--help", "-h":
		_, err := io.WriteString(env.Stdout, usage)
		return err
	}
	return usageErr(env, fmt.Errorf("unknown command %q", cmd))
}

func usageErr(env Env, err error) error {
	_, _ = io.WriteString(env.Stderr, usage)
	if err != nil {
		return err
	}
	return errUsage
}

// parse は args を位置引数と、known に載ったフラグに分ける。フラグは位置引数の後ろにも置ける。
func parse(args []string, known map[string]string) ([]string, map[string]bool, error) {
	var pos []string
	flags := map[string]bool{}
	for _, a := range args {
		if strings.HasPrefix(a, "-") && a != "-" {
			name, ok := known[a]
			if !ok {
				return nil, nil, fmt.Errorf("unknown flag %s", a)
			}
			flags[name] = true
			continue
		}
		pos = append(pos, a)
	}
	return pos, flags, nil
}

func list(env Env, all bool) error {
	var paths []string
	if all {
		root, err := layout.WorktreeRoot(env.Dir)
		if err != nil {
			return err
		}
		if paths, err = worktree.ListAll(root); err != nil {
			return err
		}
	} else {
		entries, err := worktree.List(env.Dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			paths = append(paths, e.Path)
		}
	}
	for _, p := range paths {
		if _, err := fmt.Fprintln(env.Stdout, p); err != nil {
			return err
		}
	}
	return nil
}

func hookCreate(env Env) error {
	var in struct {
		Cwd  string `json:"cwd"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(env.Stdin).Decode(&in); err != nil {
		return fmt.Errorf("reading hook input: %w", err)
	}
	if in.Cwd == "" || in.Name == "" {
		return errors.New("hook input needs cwd and name")
	}
	warnStaleSkill(env)
	path, err := worktree.Add(in.Cwd, "worktree-"+in.Name, "", env.Stderr)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(env.Stdout, path)
	return err
}

func hookRemove(env Env) error {
	var in struct {
		WorktreePath string `json:"worktree_path"`
	}
	if err := json.NewDecoder(env.Stdin).Decode(&in); err != nil {
		return fmt.Errorf("reading hook input: %w", err)
	}
	if in.WorktreePath == "" {
		return errors.New("hook input needs worktree_path")
	}
	if _, err := os.Stat(in.WorktreePath); os.IsNotExist(err) {
		return nil
	}
	return worktree.Remove(in.WorktreePath, in.WorktreePath, true, env.Stderr)
}

func warnStaleSkill(env Env) {
	if env.Version == "dev" {
		return
	}
	dir, err := claude.ConfigDir()
	if err != nil {
		return
	}
	if v := claude.InstalledSkillVersion(dir); v != "" && v != env.Version {
		_, _ = fmt.Fprintf(env.Stderr, "warning: the installed skill is from ghq-worktree %s; run `ghq-worktree install claude-code` to update it to %s\n", v, env.Version)
	}
}
