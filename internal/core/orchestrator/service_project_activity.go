package orchestrator

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

const (
	maxSurfaceProjectActivityEntries = 120
	projectCockpitRecentLimit        = 8
	projectInterjectTTL              = 10 * time.Minute
	projectContinuePrompt            = "继续当前项目，先简要确认当前状态，然后按最合理的下一步继续推进。"
)

type ProjectActivityStore interface {
	Append(control.ProjectActivityEntry)
	Recent(surfaceID string, limit int) []control.ProjectActivityEntry
}

func (s *Service) SetProjectActivityStore(store ProjectActivityStore) {
	s.projectActivityStore = store
}

func (s *Service) appendProjectActivity(surface *state.SurfaceConsoleRecord, entry control.ProjectActivityEntry) {
	if surface == nil {
		return
	}
	now := s.now().UTC()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.SurfaceSessionID = strings.TrimSpace(firstNonEmpty(entry.SurfaceSessionID, surface.SurfaceSessionID))
	entry.Workspace = strings.TrimSpace(firstNonEmpty(entry.Workspace, s.projectWorkspaceLabel(surface)))
	entry.ThreadID = strings.TrimSpace(entry.ThreadID)
	entry.TurnID = strings.TrimSpace(entry.TurnID)
	entry.QueueItemID = strings.TrimSpace(entry.QueueItemID)
	entry.Label = strings.TrimSpace(entry.Label)
	entry.Text = strings.TrimSpace(entry.Text)
	entry.Detail = strings.TrimSpace(entry.Detail)
	if len(surface.ProjectActivity) != 0 {
		last := surface.ProjectActivity[len(surface.ProjectActivity)-1]
		if last.Kind == string(entry.Kind) &&
			strings.TrimSpace(last.Label) == entry.Label &&
			strings.TrimSpace(last.Text) == entry.Text &&
			strings.TrimSpace(last.Detail) == entry.Detail &&
			strings.TrimSpace(last.ThreadID) == entry.ThreadID &&
			strings.TrimSpace(last.TurnID) == entry.TurnID &&
			strings.TrimSpace(last.QueueItemID) == entry.QueueItemID {
			surface.ProjectActivity[len(surface.ProjectActivity)-1].CreatedAt = entry.CreatedAt
			return
		}
	}
	if entry.ID == "" {
		s.nextProjectActivityID++
		entry.ID = fmt.Sprintf("activity-%d", s.nextProjectActivityID)
	}
	record := state.ProjectActivityRecord{
		ID:               entry.ID,
		SurfaceSessionID: entry.SurfaceSessionID,
		Workspace:        entry.Workspace,
		ThreadID:         entry.ThreadID,
		TurnID:           entry.TurnID,
		QueueItemID:      entry.QueueItemID,
		Kind:             string(entry.Kind),
		Label:            entry.Label,
		Text:             entry.Text,
		Detail:           entry.Detail,
		CreatedAt:        entry.CreatedAt,
	}
	surface.ProjectActivity = append(surface.ProjectActivity, record)
	if len(surface.ProjectActivity) > maxSurfaceProjectActivityEntries {
		surface.ProjectActivity = append([]state.ProjectActivityRecord(nil), surface.ProjectActivity[len(surface.ProjectActivity)-maxSurfaceProjectActivityEntries:]...)
	}
	if s.projectActivityStore != nil {
		s.projectActivityStore.Append(entry)
	}
}

func (s *Service) projectActivityEntries(surface *state.SurfaceConsoleRecord, limit int) []control.ProjectActivityEntry {
	if surface == nil || limit <= 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]control.ProjectActivityEntry, 0, limit)
	add := func(entry control.ProjectActivityEntry) {
		if len(out) >= limit {
			return
		}
		key := strings.TrimSpace(entry.ID)
		if key == "" {
			key = fmt.Sprintf("%s/%s/%s/%s", entry.SurfaceSessionID, entry.Kind, entry.CreatedAt.Format(time.RFC3339Nano), entry.Text)
		}
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, entry)
	}
	for i := len(surface.ProjectActivity) - 1; i >= 0; i-- {
		add(projectActivityEntryFromRecord(surface.ProjectActivity[i]))
	}
	if s.projectActivityStore != nil && len(out) < limit {
		for _, entry := range s.projectActivityStore.Recent(surface.SurfaceSessionID, limit) {
			add(entry)
		}
	}
	return out
}

func projectActivityEntryFromRecord(record state.ProjectActivityRecord) control.ProjectActivityEntry {
	return control.ProjectActivityEntry{
		ID:               record.ID,
		SurfaceSessionID: record.SurfaceSessionID,
		Workspace:        record.Workspace,
		ThreadID:         record.ThreadID,
		TurnID:           record.TurnID,
		QueueItemID:      record.QueueItemID,
		Kind:             control.ProjectActivityKind(record.Kind),
		Label:            record.Label,
		Text:             record.Text,
		Detail:           record.Detail,
		CreatedAt:        record.CreatedAt,
	}
}

func (s *Service) showProjectCockpit(surface *state.SurfaceConsoleRecord) []eventcontract.Event {
	event := eventcontract.Event{
		Kind:               eventcontract.KindProjectCockpit,
		GatewayID:          surface.GatewayID,
		SurfaceSessionID:   surface.SurfaceSessionID,
		ProjectCockpitView: s.buildProjectCockpitView(surface),
	}
	if flow := s.markCommandLauncherTerminal(surface); flow != nil {
		event.ProjectCockpitView.MessageID = strings.TrimSpace(flow.MessageID)
	} else if messageID := strings.TrimSpace(surface.ProjectCockpitMessageID); messageID != "" {
		event.ProjectCockpitView.MessageID = messageID
	}
	return []eventcontract.Event{event}
}

func (s *Service) showProjectActivity(surface *state.SurfaceConsoleRecord) []eventcontract.Event {
	view := s.buildProjectCockpitView(surface)
	view.StatusLabel = "项目动态"
	view.ActivityOnly = true
	view.Entries = s.projectActivityEntries(surface, maxSurfaceProjectActivityEntries)
	if messageID := strings.TrimSpace(surface.ProjectCockpitMessageID); messageID != "" {
		view.MessageID = messageID
	} else if flow := s.markCommandLauncherTerminal(surface); flow != nil {
		view.MessageID = strings.TrimSpace(flow.MessageID)
	}
	return []eventcontract.Event{{
		Kind:               eventcontract.KindProjectCockpit,
		GatewayID:          surface.GatewayID,
		SurfaceSessionID:   surface.SurfaceSessionID,
		ProjectCockpitView: view,
	}}
}

func (s *Service) ProjectCockpitRefresh(surfaceID string) []eventcontract.Event {
	surface := s.root.Surfaces[strings.TrimSpace(surfaceID)]
	messageID := ""
	if surface != nil {
		messageID = strings.TrimSpace(surface.ProjectCockpitMessageID)
	}
	if surface == nil || messageID == "" {
		return nil
	}
	view := s.buildProjectCockpitView(surface)
	view.MessageID = messageID
	return []eventcontract.Event{{
		Kind:               eventcontract.KindProjectCockpit,
		GatewayID:          surface.GatewayID,
		SurfaceSessionID:   surface.SurfaceSessionID,
		ProjectCockpitView: view,
	}}
}

func (s *Service) RecordProjectCockpitMessage(surfaceID, messageID string) {
	surface := s.root.Surfaces[strings.TrimSpace(surfaceID)]
	if surface == nil {
		return
	}
	surface.ProjectCockpitMessageID = strings.TrimSpace(messageID)
}

func (s *Service) buildProjectCockpitView(surface *state.SurfaceConsoleRecord) *control.ProjectCockpitView {
	if surface == nil {
		return &control.ProjectCockpitView{}
	}
	inst := s.root.Instances[strings.TrimSpace(surface.AttachedInstanceID)]
	status, statusLabel := s.projectStatus(surface, inst)
	currentTask, currentStep := s.projectCurrentWork(surface, inst)
	providerProfile := s.projectProviderProfile(surface)
	pending := surface.PendingInterject
	view := &control.ProjectCockpitView{
		SurfaceSessionID: surface.SurfaceSessionID,
		Workspace:        s.projectWorkspaceLabel(surface),
		ThreadLabel:      s.projectThreadLabel(surface, inst),
		Status:           status,
		StatusLabel:      statusLabel,
		CurrentTask:      currentTask,
		CurrentStep:      currentStep,
		LastResult:       s.projectLastResult(surface),
		QueuedCount:      len(surface.QueuedQueueItemIDs),
		Running:          status == control.ProjectStatusRunning,
		Idle:             status == control.ProjectStatusIdle,
		CanStop:          status == control.ProjectStatusRunning,
		CanInterject:     status == control.ProjectStatusRunning || status == control.ProjectStatusQueued,
		InterjectActive:  pending != nil && s.now().Before(pending.ExpiresAt),
		Backend:          s.surfaceBackend(surface),
		ProviderProfile:  providerProfile,
		UpdatedAt:        s.now().UTC(),
		Entries:          s.projectActivityEntries(surface, projectCockpitRecentLimit),
	}
	view.CanContinue = view.Idle && inst != nil && strings.TrimSpace(surface.SelectedThreadID) != ""
	if pending != nil {
		view.InterjectMode = string(pending.Mode)
		view.InterjectExpires = pending.ExpiresAt
	}
	return view
}

func (s *Service) projectStatusFooter(surface *state.SurfaceConsoleRecord) string {
	if surface == nil {
		return ""
	}
	inst := s.root.Instances[strings.TrimSpace(surface.AttachedInstanceID)]
	_, label := s.projectStatus(surface, inst)
	workspace := s.projectWorkspaceLabel(surface)
	if workspace == "" {
		workspace = "未选择项目"
	}
	return fmt.Sprintf("---\n当前：%s · %s · 点「项目」查看进度", workspace, label)
}

func (s *Service) projectStatus(surface *state.SurfaceConsoleRecord, inst *state.InstanceRecord) (control.ProjectStatusKind, string) {
	if surface == nil {
		return control.ProjectStatusDetached, "未连接"
	}
	if pending := activePendingRequest(surface); pending != nil {
		_ = pending
		return control.ProjectStatusWaiting, "等待确认"
	}
	if surface.ActiveQueueItemID != "" {
		return control.ProjectStatusRunning, "正在执行"
	}
	if len(surface.QueuedQueueItemIDs) != 0 {
		return control.ProjectStatusQueued, fmt.Sprintf("排队 %d 条", len(surface.QueuedQueueItemIDs))
	}
	if strings.TrimSpace(surface.AttachedInstanceID) == "" {
		return control.ProjectStatusDetached, "未连接"
	}
	if inst == nil || !inst.Online {
		return control.ProjectStatusOffline, "离线"
	}
	if surface.RouteMode == state.RouteModeNewThreadReady {
		return control.ProjectStatusIdle, "新会话待命"
	}
	return control.ProjectStatusIdle, "空闲"
}

func (s *Service) projectCurrentWork(surface *state.SurfaceConsoleRecord, inst *state.InstanceRecord) (string, string) {
	if surface == nil {
		return "", ""
	}
	if item := surface.QueueItems[strings.TrimSpace(surface.ActiveQueueItemID)]; item != nil {
		task := strings.TrimSpace(firstNonEmpty(item.SourceMessagePreview, item.ReplyToMessagePreview))
		step := projectProgressStep(surface.ActiveExecProgress)
		return task, step
	}
	if len(surface.QueuedQueueItemIDs) != 0 {
		item := surface.QueueItems[surface.QueuedQueueItemIDs[0]]
		if item != nil {
			return strings.TrimSpace(firstNonEmpty(item.SourceMessagePreview, item.ReplyToMessagePreview)), "等待当前任务结束后执行"
		}
	}
	if inst != nil && strings.TrimSpace(surface.SelectedThreadID) != "" {
		if thread := inst.Threads[strings.TrimSpace(surface.SelectedThreadID)]; thread != nil {
			return strings.TrimSpace(firstNonEmpty(thread.Preview, thread.LastAssistantMessage)), ""
		}
	}
	return "", ""
}

func projectProgressStep(progress *state.ExecCommandProgressRecord) string {
	if progress == nil {
		return ""
	}
	for i := len(progress.Entries) - 1; i >= 0; i-- {
		entry := progress.Entries[i]
		if text := strings.TrimSpace(firstNonEmpty(entry.Summary, entry.Label)); text != "" {
			return text
		}
	}
	if progress.Exploration != nil {
		rows := progress.Exploration.Block.Rows
		for i := len(rows) - 1; i >= 0; i-- {
			row := rows[i]
			if text := strings.TrimSpace(firstNonEmpty(row.Summary, strings.Join(row.Items, ", "))); text != "" {
				return text
			}
		}
	}
	return ""
}

func (s *Service) projectLastResult(surface *state.SurfaceConsoleRecord) string {
	entries := s.projectActivityEntries(surface, maxSurfaceProjectActivityEntries)
	for _, entry := range entries {
		if entry.Kind == control.ProjectActivityAssistantFinal || entry.Kind == control.ProjectActivityTurnCompleted {
			return strings.TrimSpace(firstNonEmpty(entry.Text, entry.Detail))
		}
	}
	return ""
}

func (s *Service) projectWorkspaceLabel(surface *state.SurfaceConsoleRecord) string {
	if surface == nil {
		return ""
	}
	if inst := s.root.Instances[strings.TrimSpace(surface.AttachedInstanceID)]; inst != nil {
		if name := strings.TrimSpace(firstNonEmpty(inst.DisplayName, inst.ShortName)); name != "" {
			return name
		}
		return state.WorkspaceShortName(firstNonEmpty(inst.WorkspaceKey, inst.WorkspaceRoot))
	}
	if workspace := strings.TrimSpace(s.surfaceCurrentWorkspaceKey(surface)); workspace != "" {
		return state.WorkspaceShortName(workspace)
	}
	return ""
}

func (s *Service) projectThreadLabel(surface *state.SurfaceConsoleRecord, inst *state.InstanceRecord) string {
	if surface == nil {
		return ""
	}
	threadID := strings.TrimSpace(surface.SelectedThreadID)
	if threadID == "" && inst != nil {
		threadID = strings.TrimSpace(inst.ActiveThreadID)
	}
	if inst != nil && threadID != "" {
		if thread := inst.Threads[threadID]; thread != nil {
			return displayThreadTitle(inst, thread)
		}
	}
	switch surface.RouteMode {
	case state.RouteModeNewThreadReady:
		return "新会话待命"
	case state.RouteModeUnbound:
		return "未选择会话"
	default:
		return threadID
	}
}

func (s *Service) projectProviderProfile(surface *state.SurfaceConsoleRecord) string {
	if surface == nil {
		return ""
	}
	switch s.surfaceBackend(surface) {
	case agentproto.BackendClaude:
		return s.claudeProfileDisplayName(s.surfaceClaudeProfileID(surface))
	default:
		return s.codexProviderDisplayName(s.surfaceCodexProviderID(surface))
	}
}

func (s *Service) handleProjectContinue(surface *state.SurfaceConsoleRecord, action control.Action) []eventcontract.Event {
	if surface == nil {
		return nil
	}
	continueAction := action
	continueAction.Kind = control.ActionTextMessage
	continueAction.Text = projectContinuePrompt
	continueAction.Inputs = []agentproto.Input{{Type: agentproto.InputText, Text: projectContinuePrompt}}
	continueAction.SteerInputs = continueAction.Inputs
	inst := s.root.Instances[strings.TrimSpace(surface.AttachedInstanceID)]
	status, _ := s.projectStatus(surface, inst)
	if status == control.ProjectStatusRunning || status == control.ProjectStatusQueued || status == control.ProjectStatusWaiting {
		return notice(surface, "project_continue_busy", "当前项目还在处理中。需要改变方向时点「插队」，不需要等我开新会话。")
	}
	if inst == nil {
		if events, handled := s.handlePersonalDefaultWorkspaceText(surface, continueAction, projectContinuePrompt); handled {
			return events
		}
		return notice(surface, "project_continue_detached", "当前还没有进入项目。先直接发一句你想做什么，或点「切换项目」。")
	}
	return s.handleText(surface, continueAction)
}

func (s *Service) handleProjectInterjectStart(surface *state.SurfaceConsoleRecord, action control.Action) []eventcontract.Event {
	if surface == nil {
		return nil
	}
	inst := s.root.Instances[strings.TrimSpace(surface.AttachedInstanceID)]
	status, _ := s.projectStatus(surface, inst)
	if status != control.ProjectStatusRunning && status != control.ProjectStatusQueued {
		return notice(surface, "project_interject_not_needed", "当前没有正在执行或排队的任务，直接发消息即可。")
	}
	view := &control.ProjectCockpitView{
		SurfaceSessionID: surface.SurfaceSessionID,
		Workspace:        s.projectWorkspaceLabel(surface),
		ThreadLabel:      s.projectThreadLabel(surface, inst),
		Status:           status,
		StatusLabel:      "选择插队方式",
		CurrentTask:      "下一条消息要怎么处理？",
		CurrentStep:      "选择后 10 分钟内发送一条消息即可。",
		CanInterject:     true,
		CanStop:          status == control.ProjectStatusRunning,
		Running:          status == control.ProjectStatusRunning,
		QueuedCount:      len(surface.QueuedQueueItemIDs),
		Backend:          s.surfaceBackend(surface),
		ProviderProfile:  s.projectProviderProfile(surface),
		UpdatedAt:        s.now().UTC(),
		Entries:          s.projectActivityEntries(surface, projectCockpitRecentLimit),
	}
	if messageID := strings.TrimSpace(surface.ProjectCockpitMessageID); messageID != "" {
		view.MessageID = messageID
	} else if commandCardOwnsInlineResult(action) {
		view.MessageID = strings.TrimSpace(action.MessageID)
	}
	return []eventcontract.Event{{
		Kind:               eventcontract.KindProjectCockpit,
		GatewayID:          surface.GatewayID,
		SurfaceSessionID:   surface.SurfaceSessionID,
		ProjectCockpitView: view,
	}}
}

func (s *Service) handleProjectInterjectMode(surface *state.SurfaceConsoleRecord, action control.Action) []eventcontract.Event {
	if surface == nil {
		return nil
	}
	mode := state.ProjectInterjectMode(strings.TrimSpace(control.FeishuActionArgumentText(action.Text)))
	switch mode {
	case state.ProjectInterjectModeSteer, state.ProjectInterjectModePriority:
	default:
		return notice(surface, "project_interject_mode_invalid", "请选择“补充当前任务”或“下一轮优先”。")
	}
	expiresAt := s.now().Add(projectInterjectTTL).UTC()
	surface.PendingInterject = &state.ProjectInterjectRecord{
		Mode:               mode,
		ActorUserID:        strings.TrimSpace(action.ActorUserID),
		SourceMessageID:    strings.TrimSpace(action.MessageID),
		CreatedAt:          s.now().UTC(),
		ExpiresAt:          expiresAt,
		OwnerCardMessageID: strings.TrimSpace(action.MessageID),
	}
	label := projectInterjectModeLabel(mode)
	s.appendProjectActivity(surface, control.ProjectActivityEntry{
		Kind:  control.ProjectActivityInterject,
		Label: "插队 · " + label,
		Text:  "等待下一条消息",
	})
	return notice(surface, "project_interject_ready", fmt.Sprintf("已设置：下一条消息会作为“%s”处理，10 分钟内有效。", label))
}

func (s *Service) consumePendingProjectInterject(surface *state.SurfaceConsoleRecord, action control.Action, inputs []agentproto.Input, stagedMessageIDs []string, threadID, cwd string, routeMode state.RouteMode) ([]eventcontract.Event, bool) {
	pending := surface.PendingInterject
	if pending == nil {
		return nil, false
	}
	now := s.now()
	if !pending.ExpiresAt.IsZero() && !now.Before(pending.ExpiresAt) {
		surface.PendingInterject = nil
		return nil, false
	}
	if user := strings.TrimSpace(pending.ActorUserID); user != "" && user != strings.TrimSpace(action.ActorUserID) {
		return nil, false
	}
	surface.PendingInterject = nil
	label := projectInterjectModeLabel(pending.Mode)
	s.appendProjectActivity(surface, control.ProjectActivityEntry{
		Kind:  control.ProjectActivityInterject,
		Label: "插队 · " + label,
		Text:  normalizeSourceMessagePreview(action.Text),
	})
	switch pending.Mode {
	case state.ProjectInterjectModeSteer:
		if events := s.steerCurrentTurnFromAction(surface, action, inputs); len(events) != 0 {
			return append(notice(surface, "project_interject_steer_sent", "已把这条消息补充进当前任务。"), events...), true
		}
		return append(
			notice(surface, "project_interject_downgraded", "当前任务已经不能直接补充了，这条消息已放到下一轮优先。"),
			s.enqueueQueueItem(surface, action.MessageID, action.Text, stagedMessageIDs, inputs, threadID, cwd, routeMode, surface.PromptOverride, true)...,
		), true
	case state.ProjectInterjectModePriority:
		return append(
			notice(surface, "project_interject_priority_queued", "已放到下一轮优先。"),
			s.enqueueQueueItem(surface, action.MessageID, action.Text, stagedMessageIDs, inputs, threadID, cwd, routeMode, surface.PromptOverride, true)...,
		), true
	default:
		return nil, false
	}
}

func (s *Service) steerCurrentTurnFromAction(surface *state.SurfaceConsoleRecord, action control.Action, inputs []agentproto.Input) []eventcontract.Event {
	inst, activeThreadID, activeTurnID, ok := s.activeSurfaceRemoteSteerTarget(surface)
	if !ok || len(inputs) == 0 {
		return nil
	}
	s.nextQueueItemID++
	queueItemID := "queue-" + strconv.Itoa(s.nextQueueItemID)
	cwd := strings.TrimSpace(firstNonEmpty(queueItemFrozenCWD(surface.QueueItems[surface.ActiveQueueItemID]), s.projectWorkspaceKey(surface)))
	dispatchPlan := agentproto.DefaultPromptDispatchPlanForExecutionThread(activeThreadID)
	dispatchPlan.CWD = cwd
	dispatchPlan = agentproto.NormalizePromptDispatchPlan(dispatchPlan)
	steerInputs := append([]agentproto.Input(nil), action.SteerInputs...)
	if len(steerInputs) == 0 {
		steerInputs = append([]agentproto.Input(nil), inputs...)
	}
	item := &state.QueueItemRecord{
		ID:                    queueItemID,
		SurfaceSessionID:      surface.SurfaceSessionID,
		ActorUserID:           action.ActorUserID,
		SourceKind:            state.QueueItemSourceUser,
		SourceMessageID:       action.MessageID,
		SourceMessagePreview:  normalizeSourceMessagePreview(action.Text),
		SourceMessageIDs:      uniqueStrings([]string{action.MessageID}),
		ReplyToMessageID:      action.MessageID,
		ReplyToMessagePreview: normalizeSourceMessagePreview(action.Text),
		Inputs:                append([]agentproto.Input(nil), inputs...),
		SteerInputs:           steerInputs,
		FrozenDispatchPlan:    dispatchPlan,
		FrozenOverride:        surface.PromptOverride,
		RouteModeAtEnqueue:    surface.RouteMode,
		Status:                state.QueueItemSteering,
	}
	surface.QueueItems[item.ID] = item
	if activeThreadID != "" {
		s.recordThreadUserMessage(inst, activeThreadID, action.Text)
	}
	s.turns.pendingSteers[item.ID] = &pendingSteerBinding{
		InstanceID:       inst.InstanceID,
		SurfaceSessionID: surface.SurfaceSessionID,
		QueueItemID:      item.ID,
		QueueItemIDs:     []string{item.ID},
		QueueIndices:     map[string]int{item.ID: 0},
		SourceMessageID:  action.MessageID,
		ThreadID:         activeThreadID,
		TurnID:           activeTurnID,
		QueueIndex:       0,
	}
	events := s.pendingInputEvents(surface, control.PendingInputState{
		QueueItemID: item.ID,
		Status:      string(item.Status),
		QueueOn:     true,
	}, []string{action.MessageID})
	events = append(events, eventcontract.Event{
		Kind:             eventcontract.KindAgentCommand,
		SurfaceSessionID: surface.SurfaceSessionID,
		Command: &agentproto.Command{
			Kind: agentproto.CommandTurnSteer,
			Origin: agentproto.Origin{
				Surface:   surface.SurfaceSessionID,
				UserID:    queuedItemActorUserID(item, surface),
				ChatID:    surface.ChatID,
				MessageID: action.MessageID,
			},
			Target: agentproto.Target{
				ThreadID: activeThreadID,
				TurnID:   activeTurnID,
			},
			Prompt: agentproto.Prompt{
				Inputs: queueItemSteerInputs(item),
			},
		},
	})
	return events
}

func (s *Service) activeSurfaceRemoteSteerTarget(surface *state.SurfaceConsoleRecord) (*state.InstanceRecord, string, string, bool) {
	if surface == nil || strings.TrimSpace(surface.ActiveQueueItemID) == "" {
		return nil, "", "", false
	}
	inst := s.root.Instances[strings.TrimSpace(surface.AttachedInstanceID)]
	if inst == nil || !inst.Online {
		return nil, "", "", false
	}
	binding := s.turns.activeRemote[inst.InstanceID]
	if binding == nil || strings.TrimSpace(binding.SurfaceSessionID) != strings.TrimSpace(surface.SurfaceSessionID) {
		return nil, "", "", false
	}
	threadID := strings.TrimSpace(binding.ThreadID)
	turnID := strings.TrimSpace(binding.TurnID)
	if threadID == "" || turnID == "" || s.progress.isCompactTurn(inst.InstanceID, threadID, turnID) {
		return nil, "", "", false
	}
	return inst, threadID, turnID, true
}

func projectInterjectModeLabel(mode state.ProjectInterjectMode) string {
	switch mode {
	case state.ProjectInterjectModeSteer:
		return "补充当前任务"
	case state.ProjectInterjectModePriority:
		return "下一轮优先"
	default:
		return "插队"
	}
}

func projectTurnCompletedLabel(outcome *remoteTurnOutcome) string {
	if outcome == nil {
		return "任务结束"
	}
	switch outcome.Cause {
	case terminalCauseCompleted:
		return "任务完成"
	case terminalCauseUserInterrupted:
		return "已停止"
	default:
		return "任务失败"
	}
}

func projectTurnCompletedText(outcome *remoteTurnOutcome) string {
	if outcome == nil {
		return ""
	}
	if text := strings.TrimSpace(previewOfText(outcome.FinalText)); text != "" {
		return text
	}
	if outcome.Summary != nil && outcome.Summary.FileCount > 0 {
		return fmt.Sprintf("修改了 %d 个文件", outcome.Summary.FileCount)
	}
	if msg := strings.TrimSpace(outcome.ErrorMessage); msg != "" {
		return msg
	}
	switch outcome.Cause {
	case terminalCauseCompleted:
		return "已完成。"
	case terminalCauseUserInterrupted:
		return "已停止当前任务。"
	default:
		return "任务未完成。"
	}
}

func (s *Service) expireProjectInterject(surface *state.SurfaceConsoleRecord, now time.Time) {
	if surface == nil || surface.PendingInterject == nil {
		return
	}
	if !surface.PendingInterject.ExpiresAt.IsZero() && !now.Before(surface.PendingInterject.ExpiresAt) {
		surface.PendingInterject = nil
	}
}

func (s *Service) projectWorkspaceKey(surface *state.SurfaceConsoleRecord) string {
	if surface == nil {
		return ""
	}
	if inst := s.root.Instances[strings.TrimSpace(surface.AttachedInstanceID)]; inst != nil {
		return strings.TrimSpace(firstNonEmpty(inst.WorkspaceKey, inst.WorkspaceRoot))
	}
	return strings.TrimSpace(s.surfaceCurrentWorkspaceKey(surface))
}
