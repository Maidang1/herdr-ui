package icons

import (
	"github.com/egoist/mygo/ui"
	"strings"
)

// Agent uses explicit known identities. Unknown agents retain their real name
// and use Bot rather than pretending to be a similar brand.
func Agent(name string) *ui.SVG {
	switch strings.ToLower(name) {
	case "claude", "claude-code", "claudecode":
		return ClaudeCode
	case "codex":
		return Codex
	case "gemini", "gemini-cli":
		return GeminiCLI
	case "opencode":
		return Opencode
	case "github-copilot", "copilot":
		return GithubCopilot
	case "cursor":
		return Cursor
	case "kimi":
		return Kimi
	case "":
		return Terminal
	default:
		return Bot
	}
}
