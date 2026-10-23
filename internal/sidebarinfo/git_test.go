package sidebarinfo

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func gitTest(t *testing.T, path string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", path}, args...)...)
	cmd.Env = gitEnvironment(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func TestCollectRealGitBranchDivergenceAndRemoteWithoutNetwork(t *testing.T) {
	path := t.TempDir()
	gitTest(t, path, "init", "-b", "main")
	gitTest(t, path, "config", "user.name", "Sidebar test")
	gitTest(t, path, "config", "user.email", "sidebar@example.invalid")
	gitTest(t, path, "commit", "--allow-empty", "-m", "base")
	base := gitTest(t, path, "rev-parse", "HEAD")
	gitTest(t, path, "remote", "add", "origin", "https://user:secret@github.com/org/repo.git")
	gitTest(t, path, "update-ref", "refs/remotes/origin/main", base)
	gitTest(t, path, "branch", "--set-upstream-to=origin/main")
	gitTest(t, path, "commit", "--allow-empty", "-m", "local")
	remote := gitTest(t, path, "commit-tree", "HEAD^{tree}", "-p", base, "-m", "remote")
	gitTest(t, path, "update-ref", "refs/remotes/origin/main", remote)
	t.Setenv("GIT_DIR", "/not/the/repo")
	t.Setenv("GIT_WORK_TREE", "/wrong")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "remote.origin.url")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://gitlab.com/wrong")
	info := collectGit(context.Background(), path, runCommand)
	if !info.GitKnown || info.Branch != "main" || info.Ahead != 1 || info.Behind != 1 || info.Remote != "github" {
		t.Fatal(info)
	}
	gitTest(t, path, "checkout", "--detach", "HEAD")
	if detached := collectGit(context.Background(), path, runCommand); detached.GitKnown || detached.Branch != "" {
		t.Fatal("invented detached branch", detached)
	}
	if missing := collectGit(context.Background(), t.TempDir(), runCommand); missing.GitKnown || missing.Branch != "" {
		t.Fatal("invented repository", missing)
	}
}
func TestRemoteKindsOnlyClassifyRealHost(t *testing.T) {
	for remote, want := range map[string]string{"git@github.com:a/b.git": "github", "ssh://git@gitlab.com/a/b": "gitlab", "https://github.com.evil.invalid/a": "git", "/local/repo": "git"} {
		if got := remoteKind(remote); got != want {
			t.Fatal(remote, got)
		}
	}
}
func TestBoundedOutputAndCommandCancellation(t *testing.T) {
	buffer := &limitedBuffer{limit: 3}
	if _, err := buffer.Write([]byte("four")); err == nil || buffer.Len() != 0 {
		t.Fatal("output limit bypassed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := runCommand(ctx, "/bin/sleep", "10"); err == nil || time.Since(start) > time.Second {
		t.Fatal("command ignored cancellation", err)
	}
}
