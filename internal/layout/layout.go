// Package layout は worktree をどこに置くかを決める。
package layout

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sudame/ghq-worktree/internal/git"
)

// SanitizeBranch はブランチ名をディレクトリ名に使える形にする。gwq の既定と同じ置き換え。
func SanitizeBranch(branch string) string {
	return strings.NewReplacer("/", "-", ":", "-").Replace(branch)
}

// Canonical は絶対パスにし、シンボリックリンクを解決する。パスが無ければ絶対パスだけ返す。
func Canonical(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	return abs
}

func expandHome(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, p[1:]), nil
}

func splitLines(s string) []string {
	var out []string
	for line := range strings.SplitSeq(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// GhqRoots は ghq のルートを返す。ghq があればそれに聞き、無ければ ghq と同じ順で設定を読む。
func GhqRoots() ([]string, error) {
	var raw []string
	if _, err := exec.LookPath("ghq"); err == nil {
		out, err := exec.Command("ghq", "root", "--all").Output()
		if err != nil {
			return nil, fmt.Errorf("ghq root --all: %w", err)
		}
		raw = splitLines(string(out))
	} else if v := os.Getenv("GHQ_ROOT"); v != "" {
		raw = filepath.SplitList(v)
	} else if out, err := git.Run("", "config", "--get-all", "ghq.root"); err == nil && out != "" {
		raw = splitLines(out)
	} else {
		raw = []string{"~/ghq"}
	}

	roots := make([]string, 0, len(raw))
	for _, r := range raw {
		p, err := expandHome(r)
		if err != nil {
			return nil, err
		}
		roots = append(roots, Canonical(p))
	}
	return roots, nil
}

// WorktreeRoot は worktree を置くディレクトリを返す。git config の ghq-worktree.root、無ければ ~/worktrees。
func WorktreeRoot(repo string) (string, error) {
	root := "~/worktrees"
	if out, err := git.Run(repo, "config", "--get", "ghq-worktree.root"); err == nil && out != "" {
		root = out
	}
	p, err := expandHome(root)
	if err != nil {
		return "", err
	}
	return Canonical(p), nil
}

// RepoRoot は dir が属するリポジトリの、元の作業ディレクトリを返す。dir が worktree の中でも元を返す。
func RepoRoot(dir string) (string, error) {
	out, err := git.Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	common := Canonical(out)
	if filepath.Base(common) != ".git" {
		return "", fmt.Errorf("%s: only repositories with a .git directory are supported", common)
	}
	return filepath.Dir(common), nil
}

// RelToGhq は repo を、それを含む ghq のルートからの相対パスにする。
func RelToGhq(repo string, roots []string) (string, error) {
	for _, root := range roots {
		rel, err := filepath.Rel(root, repo)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return rel, nil
	}
	return "", fmt.Errorf("%s is not under any ghq root (%s)", repo, strings.Join(roots, ", "))
}

// WorktreePath は repo の branch を置く worktree のパスを返す。
func WorktreePath(repo, branch string) (string, error) {
	roots, err := GhqRoots()
	if err != nil {
		return "", err
	}
	rel, err := RelToGhq(repo, roots)
	if err != nil {
		return "", err
	}
	wroot, err := WorktreeRoot(repo)
	if err != nil {
		return "", err
	}
	return filepath.Join(wroot, rel, SanitizeBranch(branch)), nil
}
