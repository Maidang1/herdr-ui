package icons

import "testing"

func TestBrandUsesTheColorMarks(t *testing.T) {
	for _, name := range []string{"claude", "claude-code", "claudecode", "codex", "gemini", "gemini-cli", "opencode", "github-copilot", "copilot", "cursor", "kimi", "grok", "pi", "devin", "kilo", "kilo-code", "qwen", "antigravity", "antigravity_cli"} {
		mark, ok := Brand(name, false)
		if !ok || mark == nil {
			t.Errorf("%s has no color mark", name)
		}
	}
	if mark, ok := Brand("shell", false); ok || mark != nil {
		t.Fatal("an unknown agent got a brand mark")
	}
	if Agent("") != Terminal || Agent("shell") != Bot {
		t.Fatal("fallback marks changed")
	}
	if light, _ := Brand("claude", false); light != ClaudeCode {
		t.Fatal("claude light mark is not the color logo")
	}
	for _, name := range []string{"claude", "gemini", "gemini-cli", "opencode", "codex", "copilot", "github-copilot", "devin", "qwen", "antigravity"} {
		light, _ := Brand(name, false)
		dark, _ := Brand(name, true)
		if light != dark {
			t.Errorf("%s changed in a dark window, but its logo has no black ink", name)
		}
	}
	for _, name := range []string{"cursor", "kimi", "grok", "pi", "kilo"} {
		light, _ := Brand(name, false)
		dark, _ := Brand(name, true)
		if light == dark {
			t.Errorf("%s kept black ink in a dark window", name)
		}
	}
}
