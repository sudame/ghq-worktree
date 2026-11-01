// Package worktree は worktree の作成・削除・一覧を行う。
package worktree

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sudame/ghq-worktree/internal/git"
	"github.com/sudame/ghq-worktree/internal/layout"
)

// Entry は git worktree list の 1 件。
type Entry struct {
	Path   string
	Branch string
}

// Add は dir のリポジトリに branch の worktree を作り、そのパスを返す。
// branch が無ければ start（空なら dir の HEAD）から作る。進捗は log に書く。
func Add(dir, branch, start string, log io.Writer) (string, error) {
	repo, err := layout.RepoRoot(dir)
	if err != nil {
		return "", err
	}
	path, err := layout.WorktreePath(repo, branch)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("%s already exists", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	if branchExists(dir, branch) {
		if start != "" {
			return "", fmt.Errorf("branch %s already exists; a start point cannot be given", branch)
		}
		_, err = git.Run(dir, "worktree", "add", path, branch)
	} else {
		if start == "" {
			start = "HEAD"
		}
		_, err = git.Run(dir, "worktree", "add", "-b", branch, path, start)
	}
	if err != nil {
		return "", err
	}

	path = layout.Canonical(path)
	logf(log, "created %s (branch %s)\n", path, branch)
	setup(repo, path, log)
	return path, nil
}

// Existing は branch の worktree がこの CLI の置き場所にすでにあれば、そのパスを返す。
func Existing(dir, branch string) (string, bool) {
	repo, err := layout.RepoRoot(dir)
	if err != nil {
		return "", false
	}
	entries, err := List(repo)
	if err != nil {
		return "", false
	}
	p, err := layout.WorktreePath(repo, branch)
	if err != nil {
		return "", false
	}
	p = layout.Canonical(p)
	for _, e := range entries {
		if e.Path == p && e.Branch == branch {
			return p, true
		}
	}
	return "", false
}

func branchExists(dir, branch string) bool {
	_, err := git.Run(dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// Remove は target（ブランチ名か worktree のパス）の worktree を消す。
// ブランチはマージ済みのときだけ消す。
func Remove(dir, target string, force bool, log io.Writer) error {
	repo, err := layout.RepoRoot(dir)
	if err != nil {
		return err
	}
	entries, err := List(repo)
	if err != nil {
		return err
	}

	found := find(entries, repo, dir, target)
	if found == nil {
		return fmt.Errorf("no worktree of %s matches %q", repo, target)
	}
	if found.Path == repo {
		return fmt.Errorf("refusing to remove the main worktree %s", repo)
	}

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	if _, err := git.Run(repo, append(args, found.Path)...); err != nil {
		return err
	}
	logf(log, "removed %s\n", found.Path)

	deleteBranchIfMerged(repo, *found, log)
	return nil
}

// find は target に当たる worktree を探す。target はまずブランチ名として、次にパスとして見る。
func find(entries []Entry, repo, dir, target string) *Entry {
	if p, err := layout.WorktreePath(repo, target); err == nil {
		p = layout.Canonical(p)
		for i := range entries {
			// feat/x と feat-x は同じパスになるので、ブランチ名でも確かめる
			if entries[i].Path == p && entries[i].Branch == target {
				return &entries[i]
			}
		}
	}
	p := target
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	p = layout.Canonical(p)
	for i := range entries {
		if entries[i].Path == p {
			return &entries[i]
		}
	}
	return nil
}

// deleteBranchIfMerged は、worktree のブランチが元の作業ディレクトリの HEAD にマージ済みなら消す。
// worktree の中で別のブランチに切り替えていた場合は、そのブランチは消さない。
func deleteBranchIfMerged(repo string, e Entry, log io.Writer) {
	if e.Branch == "" {
		return
	}
	if layout.SanitizeBranch(e.Branch) != filepath.Base(e.Path) {
		logf(log, "kept branch %s: the worktree was not created for it\n", e.Branch)
		return
	}
	// git branch -d は upstream にマージ済みなら消してしまうので、HEAD との関係を自分で確かめる
	if _, err := git.Run(repo, "merge-base", "--is-ancestor", "refs/heads/"+e.Branch, "HEAD"); err != nil {
		logf(log, "kept branch %s: not merged into HEAD of %s\n", e.Branch, repo)
		return
	}
	if _, err := git.Run(repo, "branch", "-D", e.Branch); err != nil {
		logf(log, "kept branch %s: %v\n", e.Branch, err)
		return
	}
	logf(log, "deleted branch %s\n", e.Branch)
}

// List は dir のリポジトリの worktree を、元の作業ディレクトリも含めて返す。
func List(dir string) ([]Entry, error) {
	out, err := git.Run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for line := range strings.SplitSeq(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			entries = append(entries, Entry{Path: layout.Canonical(strings.TrimPrefix(line, "worktree "))})
		case strings.HasPrefix(line, "branch ") && len(entries) > 0:
			entries[len(entries)-1].Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		}
	}
	return entries, nil
}

// ListAll は root の下にある worktree のパスをすべて返す。
func ListAll(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root && os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if fi, err := os.Lstat(filepath.Join(p, ".git")); err == nil && fi.Mode().IsRegular() {
			paths = append(paths, p)
			return filepath.SkipDir
		}
		return nil
	})
	return paths, err
}

func setup(repo, wt string, log io.Writer) {
	if err := copyIncludes(repo, wt, log); err != nil {
		logf(log, "warning: copying .worktreeinclude files: %v\n", err)
	}
	out, err := git.Run(repo, "config", "--get-all", "ghq-worktree.setup")
	if err != nil {
		return
	}
	for c := range strings.SplitSeq(out, "\n") {
		if c = strings.TrimSpace(c); c == "" {
			continue
		}
		logf(log, "running %s\n", c)
		cmd := exec.Command("sh", "-c", c)
		cmd.Dir = wt
		cmd.Stdout = log
		cmd.Stderr = log
		if err := cmd.Run(); err != nil {
			logf(log, "warning: %s: %v\n", c, err)
		}
	}
}

// copyIncludes は .worktreeinclude に合い、かつ gitignore されているファイルを repo から wt へコピーする。
func copyIncludes(repo, wt string, log io.Writer) error {
	inc := filepath.Join(repo, ".worktreeinclude")
	if _, err := os.Stat(inc); err != nil {
		return nil
	}
	matched, err := git.Run(repo, "ls-files", "-z", "--others", "--ignored", "--exclude-from="+inc)
	if err != nil {
		return err
	}
	ignored, err := git.Run(repo, "ls-files", "-z", "--others", "--ignored", "--exclude-standard")
	if err != nil {
		return err
	}
	isIgnored := map[string]bool{}
	for f := range strings.SplitSeq(ignored, "\x00") {
		isIgnored[f] = true
	}
	for f := range strings.SplitSeq(matched, "\x00") {
		if f == "" || !isIgnored[f] {
			continue
		}
		dst := filepath.Join(wt, f)
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		if err := copyFile(filepath.Join(repo, f), dst); err != nil {
			return err
		}
		logf(log, "copied %s\n", f)
	}
	return nil
}

func copyFile(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, fi.Mode().Perm())
}

// logf は進捗を log に書く。書けなくても本処理は続ける。
func logf(log io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(log, format, args...)
}
