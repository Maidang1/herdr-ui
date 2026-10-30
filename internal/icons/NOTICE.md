# Application icon sources

Lucide and Lobe icons are generated from the exact versions in package-lock.json. SOURCES.txt records every original file and display mode. Lucide SVGs retain their ISC license and the applicable Feather-derived MIT notice in LICENSE-lucide. Lobe SVGs retain the upstream MIT license in LICENSE-lobe; its original source is vendored at assets/licenses/lobe-icons-1.98.0.LICENSE, downloaded from upstream commit 86b3f06ad9a4cc57011a56e5a7d3a5d4f1e8d835.

Agent marks for Claude Code, Codex, Gemini CLI, OpenCode, GitHub Copilot, Cursor, and Kimi are the color SVGs in assets/agent-logos. Prime Agent uses the upstream SVG from https://raw.githubusercontent.com/Untrivial-ai/agent-orchestrator/v0.13.1-nightly.202609232348/docs/assets/readme/agents/prime-agent.svg, stored as assets/agent-logos/prime-agent.svg. The generator copies them unchanged. The interface draws them with ui.Image so their fills stay. In a dark window, pure black ink in those files is drawn white; every other color stays. The GitHub repository mark remains the monochrome Lobe icon and is rendered through ui.Icon. Existing assets/icons remain design sources with their separate NOTICE.md; they are not mislabeled as generated Lucide resources.

Additional agent marks are downloaded from official sites using Chrome:

- OMP: https://omp.sh/favicon.svg (original colors).
- Droid: https://factory.com/ navigation logo (`#mobile-nav-logo`); `currentColor` is stored as black for the existing light/dark variant mechanism.
- Qoder CLI: https://qoder.com/favIcon.svg (original colors).
- Hermes: https://web-assets.nousresearch.com/marketing/076d75a3831676bc0c55edd132f5a20ba3d94cd6/assets/hermes-landing/teams/hermes-wing.svg (official wing; white fill normalized to black, original blue retained).
- Letta: https://www.letta.com/ inline logo; first two paths retained as the standalone symbol, with a square viewBox.
- Mastra Code: https://code.mastra.ai/img/mastra.svg (official Mastra symbol).

Single-color `currentColor` fills are stored as `#000000` so the existing dark-theme variant can draw them white.
