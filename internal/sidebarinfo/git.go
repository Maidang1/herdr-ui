package sidebarinfo

import (
	"context"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

func collectGit(ctx context.Context, path string, run commandFunc) Info {
	info := Info{Path: path, Remote: "none"}
	if path == "" || !filepath.IsAbs(path) {
		return info
	}
	if root, err := run(ctx, "git", "-C", path, "rev-parse", "--show-toplevel"); err == nil {
		value := strings.TrimSpace(string(root.data))
		if filepath.IsAbs(value) && !strings.ContainsAny(value, "\r\n\x00") {
			info.RepoRoot = value
		}
	}
	branch, err := run(ctx, "git", "-C", path, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return info
	}
	name := strings.TrimSpace(string(branch.data))
	if name == "" || strings.ContainsAny(name, "\r\n\x00") {
		return info
	}
	info.Branch = name
	info.GitKnown = true
	info.Remote = "git"
	if upstream, err := run(ctx, "git", "-C", path, "rev-list", "--left-right", "--count", "HEAD...@{upstream}"); err == nil {
		fields := strings.Fields(string(upstream.data))
		if len(fields) == 2 {
			ahead, aerr := strconv.Atoi(fields[0])
			behind, berr := strconv.Atoi(fields[1])
			if aerr == nil && berr == nil && ahead >= 0 && behind >= 0 {
				info.Ahead = ahead
				info.Behind = behind
			}
		}
	}
	if remote, err := run(ctx, "git", "-C", path, "config", "--get", "remote.origin.url"); err == nil {
		info.Remote = remoteKind(strings.TrimSpace(string(remote.data)))
	}
	return info
}

// Only the host category leaves this function; credentials and repository URLs
// are neither published nor logged.
func remoteKind(remote string) string {
	host := ""
	if parsed, err := url.Parse(remote); err == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	} else if !strings.Contains(remote, "://") {
		prefix, _, ok := strings.Cut(remote, ":")
		if ok {
			if _, after, ok := strings.Cut(prefix, "@"); ok {
				prefix = after
			}
			host = prefix
		}
	}
	switch strings.ToLower(host) {
	case "github.com", "www.github.com":
		return "github"
	case "gitlab.com", "www.gitlab.com":
		return "gitlab"
	default:
		return "git"
	}
}
