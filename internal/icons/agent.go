package icons

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"
)

// blackFill matches pure black ink. A dark window draws that ink white.
var blackFill = regexp.MustCompile(`(?i)\bfill=(?:"(?:#000000|#000|black)"|'(?:#000000|#000|black)')`)

var (
	claudeDark      *ui.SVG
	codexDark       *ui.SVG
	geminiDark      *ui.SVG
	opencodeDark    *ui.SVG
	copilotDark     *ui.SVG
	cursorDark      *ui.SVG
	kimiDark        *ui.SVG
	grokDark        *ui.SVG
	piDark          *ui.SVG
	devinDark       *ui.SVG
	kiloDark        *ui.SVG
	qwenDark        *ui.SVG
	antigravityDark *ui.SVG
)

func init() {
	claudeDark = darkVariant("color-claude", ClaudeCode)
	codexDark = darkVariant("color-codex", Codex)
	geminiDark = darkVariant("color-gemini", GeminiCLI)
	opencodeDark = darkVariant("color-opencode", Opencode)
	copilotDark = darkVariant("color-github-copilot", GithubCopilot)
	cursorDark = darkVariant("color-cursor", Cursor)
	kimiDark = darkVariant("color-kimi", Kimi)
	grokDark = darkVariant("color-grok", Grok)
	piDark = darkVariant("color-pi", Pi)
	devinDark = darkVariant("color-devin", Devin)
	kiloDark = darkVariant("color-kilo-code", Kilo)
	qwenDark = darkVariant("color-qwen", Qwen)
	antigravityDark = darkVariant("color-antigravity", Antigravity)
}

func darkVariant(file string, light *ui.SVG) *ui.SVG {
	data, err := resources.ReadFile("svg/" + file + ".svg")
	if err != nil {
		panic(err)
	}
	ink := blackFill.ReplaceAll(data, []byte(`fill="#FFFFFF"`))
	if bytes.Equal(ink, data) {
		return light
	}
	return ui.MustParseSVG(ink)
}

// Brand returns the color logo for a known agent. In a dark window, pure
// black ink is drawn white so the mark stays visible. Every other color
// stays as it is in the logo.
func Brand(name string, dark bool) (*ui.SVG, bool) {
	light, lifted := brandPair(name)
	if light == nil {
		return nil, false
	}
	if dark {
		return lifted, true
	}
	return light, true
}

func brandPair(name string) (light, dark *ui.SVG) {
	switch strings.ToLower(name) {
	case "claude", "claude-code", "claudecode":
		return ClaudeCode, claudeDark
	case "codex":
		return Codex, codexDark
	case "gemini", "gemini-cli":
		return GeminiCLI, geminiDark
	case "opencode":
		return Opencode, opencodeDark
	case "github-copilot", "copilot":
		return GithubCopilot, copilotDark
	case "cursor":
		return Cursor, cursorDark
	case "kimi":
		return Kimi, kimiDark
	case "grok":
		return Grok, grokDark
	case "pi":
		return Pi, piDark
	case "devin":
		return Devin, devinDark
	case "kilo", "kilo-code", "kilocode":
		return Kilo, kiloDark
	case "qwen":
		return Qwen, qwenDark
	case "antigravity", "antigravity-cli", "antigravity_cli":
		return Antigravity, antigravityDark
	default:
		return nil, nil
	}
}

// Agent uses explicit known identities. Unknown agents retain their real name
// and use Bot rather than pretending to be a similar brand.
func Agent(name string) *ui.SVG {
	if mark, ok := Brand(name, false); ok {
		return mark
	}
	if name == "" {
		return Terminal
	}
	return Bot
}
