package detectors

import "github.com/ttime-ai/ttime/client/internal/scanner"

func init() {
	for _, def := range agentsViewGenericDefinitions() {
		def := def
		scanner.Register(func() scanner.Detector {
			return newAgentsViewGenericDetector(def)
		})
	}
}

func agentsViewGenericDefinitions() []agentsViewGenericDefinition {
	return []agentsViewGenericDefinition{
		{Name: "copilot", Description: "GitHub Copilot sessions", Paths: []string{"~/.copilot"}},
		{Name: "openhands", Description: "OpenHands CLI sessions", Paths: []string{"~/.openhands/conversations"}},
		{Name: "iflow", Description: "iFlow sessions", Paths: []string{"~/.iflow/projects"}},
		{Name: "amp", Description: "Amp sessions", Paths: []string{"~/.local/share/amp/threads"}},
		{Name: "zencoder", Description: "Zencoder sessions", Paths: []string{"~/.zencoder/sessions"}},
		{Name: "vscode_copilot", Description: "VS Code Copilot chat sessions", Paths: []string{
			"~/Library/Application Support/Code/User/workspaceStorage",
			"~/Library/Application Support/Code/User/globalStorage",
			"~/.config/Code/User/workspaceStorage",
			"~/.config/Code/User/globalStorage",
			"~/AppData/Roaming/Code/User/workspaceStorage",
			"~/AppData/Roaming/Code/User/globalStorage",
		}},
		{Name: "pi", Description: "Pi agent sessions", Paths: []string{"~/.pi/agent/sessions"}},
		{Name: "qwen", Description: "Qwen Code sessions", Paths: []string{"~/.qwen/projects"}},
		{Name: "commandcode", Description: "Command Code sessions", Paths: []string{"~/.commandcode/projects"}},
		{Name: "qclaw", Description: "QClaw sessions", Paths: []string{"~/.qclaw/agents"}},
		{Name: "kimi", Description: "Kimi sessions", Paths: []string{"~/.kimi/sessions"}},
		{Name: "claude_ai", Description: "Claude.ai exports", Paths: []string{"~/.claude-ai", "~/.config/claude-ai"}},
		{Name: "chatgpt", Description: "ChatGPT exports", Paths: []string{"~/.chatgpt", "~/.config/chatgpt"}},
		{Name: "kiro", Description: "Kiro sessions", Paths: []string{"~/.kiro/sessions/cli", "~/.local/share/kiro-cli"}},
		{Name: "kiro_ide", Description: "Kiro IDE sessions", Paths: []string{
			"~/Library/Application Support/Kiro/User/workspaceStorage",
			"~/.config/Kiro/User/workspaceStorage",
			"~/AppData/Roaming/Kiro/User/workspaceStorage",
		}},
		{Name: "cortex", Description: "Cortex Code sessions", Paths: []string{"~/.snowflake/cortex/conversations"}},
		{Name: "workbuddy", Description: "WorkBuddy sessions", Paths: []string{"~/.workbuddy/projects"}},
		{Name: "forge", Description: "Forge sessions", Paths: []string{"~/.forge"}},
		{Name: "piebald", Description: "Piebald sessions", Paths: []string{"~/.piebald", "~/.local/share/piebald"}},
		{Name: "warp", Description: "Warp sessions", Paths: []string{"~/Library/Application Support/dev.warp.Warp-Stable", "~/.local/share/warp-terminal"}},
		{Name: "positron", Description: "Positron Assistant sessions", Paths: []string{
			"~/Library/Application Support/Positron/User/workspaceStorage",
			"~/.config/Positron/User/workspaceStorage",
		}},
		{Name: "antigravity", Description: "Antigravity sessions", Paths: []string{"~/.gemini/antigravity"}},
		{Name: "antigravity_cli", Description: "Antigravity CLI sessions", Paths: []string{"~/.gemini/antigravity-cli"}},
		{Name: "zed", Description: "Zed assistant sessions", Paths: []string{"~/.config/zed/threads", "~/Library/Application Support/Zed/threads"}},
	}
}
