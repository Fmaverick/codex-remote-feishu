package feishu

import (
	"fmt"
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
)

func (p *Projector) projectProjectCockpit(chatID string, event eventcontract.Event, view control.ProjectCockpitView) []Operation {
	daemonLifecycleID := firstNonEmpty(event.DaemonLifecycleID, event.Meta.DaemonLifecycleID)
	elements := projectCockpitElements(view, daemonLifecycleID)
	op := Operation{
		Kind:             OperationSendCard,
		GatewayID:        event.GatewayID,
		SurfaceSessionID: event.SurfaceSessionID,
		ChatID:           chatID,
		MessageID:        strings.TrimSpace(view.MessageID),
		CardTitle:        "项目",
		CardBody:         "",
		CardThemeKey:     projectCockpitTheme(view),
		CardElements:     elements,
		CardUpdateMulti:  true,
		cardEnvelope:     cardEnvelopeV2,
		card:             rawCardDocument("项目", "", projectCockpitTheme(view), elements),
	}
	if strings.TrimSpace(op.MessageID) != "" {
		op.Kind = OperationUpdateCard
	} else {
		op = applyReplyLaneToNewOperation(event, op)
	}
	return []Operation{op}
}

func projectCockpitElements(view control.ProjectCockpitView, daemonLifecycleID string) []map[string]any {
	var elements []map[string]any
	if view.ActivityOnly {
		header := []string{
			fmt.Sprintf("**当前项目** %s", projectCockpitValue(view.Workspace, "未选择")),
			"**视图** 项目动态",
		}
		elements = append(elements, cardMarkdownLines(header)...)
		if buttons := projectCockpitActionButtons(view, daemonLifecycleID); len(buttons) != 0 {
			elements = append(elements, cardDividerElement(), cardButtonGroupElement(buttons))
		}
		elements = append(elements, projectCockpitActivityElements(view.Entries, "完整动态")...)
		return elements
	}
	header := []string{
		fmt.Sprintf("**当前项目** %s", projectCockpitValue(view.Workspace, "未选择")),
		fmt.Sprintf("**状态** %s", projectCockpitValue(view.StatusLabel, "未知")),
	}
	if thread := strings.TrimSpace(view.ThreadLabel); thread != "" {
		header = append(header, fmt.Sprintf("**会话** %s", thread))
	}
	if provider := strings.TrimSpace(view.ProviderProfile); provider != "" {
		header = append(header, fmt.Sprintf("**配置** %s · %s", strings.ToLower(string(agentproto.NormalizeBackend(view.Backend))), provider))
	}
	elements = append(elements, cardMarkdownLines(header)...)
	if task := strings.TrimSpace(view.CurrentTask); task != "" {
		elements = append(elements, cardDividerElement())
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": "**正在关注** " + task,
		})
	}
	if step := strings.TrimSpace(view.CurrentStep); step != "" {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": "**当前步骤** " + step,
		})
	}
	if result := strings.TrimSpace(view.LastResult); result != "" && view.CurrentTask == "" {
		elements = append(elements, cardDividerElement())
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": "**最近结果** " + result,
		})
	}
	if view.InterjectActive {
		mode := "插队"
		switch strings.TrimSpace(view.InterjectMode) {
		case "steer":
			mode = "补充当前任务"
		case "priority":
			mode = "下一轮优先"
		}
		line := "下一条消息将按“" + mode + "”处理"
		if !view.InterjectExpires.IsZero() {
			line += "，" + projectCockpitExpiresIn(view.InterjectExpires) + "内有效"
		}
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": line,
		})
	}
	if buttons := projectCockpitActionButtons(view, daemonLifecycleID); len(buttons) != 0 {
		elements = append(elements, cardDividerElement(), cardButtonGroupElement(buttons))
	}
	elements = append(elements, projectCockpitActivityElements(view.Entries, "最近动态")...)
	return elements
}

func projectCockpitActionButtons(view control.ProjectCockpitView, daemonLifecycleID string) []map[string]any {
	var buttons []map[string]any
	if view.CanContinue {
		buttons = append(buttons, cardCallbackButtonElement("继续干活", "primary", localCardActionPayload(control.ActionProjectContinue, "", daemonLifecycleID), false, "default"))
	}
	if view.CanStop {
		buttons = append(buttons, cardCallbackButtonElement("停止", "danger", localCardActionPayload(control.ActionStop, "", daemonLifecycleID), false, "default"))
	}
	if view.CanInterject {
		buttons = append(buttons, cardCallbackButtonElement("插队", "default", localCardActionPayload(control.ActionProjectInterjectStart, "", daemonLifecycleID), false, "default"))
	}
	if strings.TrimSpace(view.StatusLabel) == "选择插队方式" {
		buttons = []map[string]any{
			cardCallbackButtonElement("补充当前任务", "primary", localCardActionPayload(control.ActionProjectInterjectMode, "steer", daemonLifecycleID), false, "default"),
			cardCallbackButtonElement("下一轮优先", "default", localCardActionPayload(control.ActionProjectInterjectMode, "priority", daemonLifecycleID), false, "default"),
		}
		if view.CanStop {
			buttons = append(buttons, cardCallbackButtonElement("停止", "danger", localCardActionPayload(control.ActionStop, "", daemonLifecycleID), false, "default"))
		}
		return buttons
	}
	if view.ActivityOnly {
		return []map[string]any{
			cardCallbackButtonElement("返回项目", "primary", localCardActionPayload(control.ActionProjectCockpit, "", daemonLifecycleID), false, "default"),
			cardCallbackButtonElement("切换项目", "default", localCardActionPayload(control.ActionListInstances, "", daemonLifecycleID), false, "default"),
		}
	}
	buttons = append(buttons,
		cardCallbackButtonElement("查看动态", "default", localCardActionPayload(control.ActionProjectActivity, "", daemonLifecycleID), false, "default"),
		cardCallbackButtonElement("切换项目", "default", localCardActionPayload(control.ActionListInstances, "", daemonLifecycleID), false, "default"),
	)
	return buttons
}

func projectCockpitActivityElements(entries []control.ProjectActivityEntry, title string) []map[string]any {
	if len(entries) == 0 {
		return nil
	}
	elements := []map[string]any{
		cardDividerElement(),
		{
			"tag":     "markdown",
			"content": "**" + strings.TrimSpace(title) + "**",
		},
	}
	for _, entry := range entries {
		line := projectCockpitActivityLine(entry)
		if line == "" {
			continue
		}
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": line,
		})
	}
	return elements
}

func cardMarkdownLines(lines []string) []map[string]any {
	elements := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			elements = append(elements, map[string]any{"tag": "markdown", "content": line})
		}
	}
	return elements
}

func projectCockpitActivityLine(entry control.ProjectActivityEntry) string {
	label := strings.TrimSpace(entry.Label)
	if label == "" {
		label = strings.TrimSpace(string(entry.Kind))
	}
	text := strings.TrimSpace(firstNonEmpty(entry.Text, entry.Detail))
	if text == "" {
		return label
	}
	if len([]rune(text)) > 80 {
		text = string([]rune(text)[:80]) + "..."
	}
	return fmt.Sprintf("- %s：%s", label, text)
}

func projectCockpitValue(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func projectCockpitTheme(view control.ProjectCockpitView) string {
	switch view.Status {
	case control.ProjectStatusRunning, control.ProjectStatusQueued:
		return cardThemeProgress
	case control.ProjectStatusWaiting:
		return cardThemeApproval
	case control.ProjectStatusOffline, control.ProjectStatusDetached:
		return cardThemeError
	default:
		return cardThemeInfo
	}
}

func projectCockpitExpiresIn(expiresAt time.Time) string {
	remaining := time.Until(expiresAt)
	if remaining <= 0 {
		return "已过期"
	}
	minutes := int(remaining.Minutes())
	if minutes <= 0 {
		return "1 分钟"
	}
	return fmt.Sprintf("%d 分钟", minutes)
}
