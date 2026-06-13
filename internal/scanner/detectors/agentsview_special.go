package detectors

import "context"

func (d *agentsViewGenericDetector) collectSpecialSummaries(ctx context.Context) ([]agentsViewGenericSummary, bool, error) {
	var (
		summaries []agentsViewGenericSummary
		err       error
	)
	switch d.Name() {
	case "claude_ai":
		summaries, err = collectClaudeAIExportSummaries(ctx, d.foundPath)
	case "chatgpt":
		summaries, err = collectChatGPTExportSummaries(ctx, d.foundPath)
	case "commandcode":
		summaries, err = collectCommandCodeGenericSummaries(ctx, d.foundPath)
	case "copilot":
		summaries, err = collectCopilotGenericSummaries(ctx, d.foundPath)
	case "kiro_ide":
		summaries, err = collectKiroIDEGenericSummaries(ctx, d.foundPath)
	case "qwen":
		summaries, err = collectQwenGenericSummaries(ctx, d.foundPath)
	case "openclaw":
		summaries, err = collectOpenClawGenericSummaries(ctx, d.foundPath)
	case "qclaw":
		summaries, err = collectQClawGenericSummaries(ctx, d.foundPath)
	case "vscode_copilot", "positron":
		summaries, err = collectVSCodeCopilotGenericSummaries(ctx, d.foundPath)
	case "hermes":
		summaries, err = collectHermesGenericSummaries(ctx, d.foundPath)
	case "workbuddy":
		summaries, err = collectWorkBuddyGenericSummaries(ctx, d.foundPath)
	case "openhands":
		summaries, err = collectOpenHandsGenericSummaries(ctx, d.foundPath)
	default:
		if !isAgentsViewSQLiteDetector(d.Name()) {
			return nil, false, nil
		}
		summaries, err = collectSQLiteGenericSummaries(ctx, d.Name(), d.foundPath)
	}
	if err != nil {
		return nil, true, err
	}
	return summaries, len(summaries) > 0 || d.Name() == "openhands", nil
}
