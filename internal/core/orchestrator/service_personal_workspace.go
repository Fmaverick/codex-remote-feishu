package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

const personalDefaultWorkspaceName = "CodexChat"

func (s *Service) handlePersonalDefaultWorkspaceText(surface *state.SurfaceConsoleRecord, action control.Action, text string) ([]eventcontract.Event, bool) {
	if !s.shouldUsePersonalDefaultWorkspace(surface) {
		return nil, false
	}
	workspaceKey, err := ensurePersonalDefaultWorkspaceRoot()
	if err != nil {
		return notice(surface, "personal_workspace_unavailable", fmt.Sprintf("个人聊天空间 ~/%s 暂时不可用：%v。请发送 /list 选择一个工作区，或检查本机目录权限。", personalDefaultWorkspaceName, err)), true
	}

	events := s.AttachWorkspaceForSurface(surface.SurfaceSessionID, workspaceKey, true)
	events = rewritePersonalDefaultWorkspaceNotices(events)
	if inst := s.root.Instances[surface.AttachedInstanceID]; inst != nil {
		return append(events, s.handleText(surface, action)...), true
	}
	if surface.PendingHeadless == nil {
		if len(events) != 0 {
			return events, true
		}
		return notice(surface, "personal_workspace_start_failed", "个人聊天空间暂时没有启动成功。请稍后再发一次消息，或发送 /list 选择一个工作区。"), true
	}

	s.rememberPersonalDefaultFirstInput(surface.PendingHeadless.InstanceID, action)
	return events, true
}

func (s *Service) shouldUsePersonalDefaultWorkspace(surface *state.SurfaceConsoleRecord) bool {
	if surface == nil || !s.surfaceIsHeadless(surface) {
		return false
	}
	if strings.TrimSpace(surface.AttachedInstanceID) != "" || surface.PendingHeadless != nil {
		return false
	}
	if !strings.Contains(strings.TrimSpace(surface.SurfaceSessionID), ":user:") {
		return false
	}
	currentWorkspace := normalizeWorkspaceClaimKey(s.surfaceCurrentWorkspaceKey(surface))
	if currentWorkspace != "" {
		return state.WorkspaceShortName(currentWorkspace) == personalDefaultWorkspaceName
	}
	return surface.RouteMode == "" || surface.RouteMode == state.RouteModeUnbound
}

func ensurePersonalDefaultWorkspaceRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(home, personalDefaultWorkspaceName)
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	return state.ResolveWorkspaceRootOnHost(root)
}

func (s *Service) rememberPersonalDefaultFirstInput(instanceID string, action control.Action) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return
	}
	if s.personalDefaultInputs == nil {
		s.personalDefaultInputs = map[string]control.Action{}
	}
	s.personalDefaultInputs[instanceID] = action
}

func (s *Service) consumePersonalDefaultFirstInput(instanceID string) (control.Action, bool) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" || s.personalDefaultInputs == nil {
		return control.Action{}, false
	}
	action, ok := s.personalDefaultInputs[instanceID]
	delete(s.personalDefaultInputs, instanceID)
	return action, ok
}

func (s *Service) enqueuePersonalDefaultFirstInput(surface *state.SurfaceConsoleRecord, workspaceKey string, action control.Action) []eventcontract.Event {
	text := strings.TrimSpace(action.Text)
	inputs := append([]agentproto.Input{}, action.Inputs...)
	if len(inputs) == 0 && text != "" {
		inputs = []agentproto.Input{{Type: agentproto.InputText, Text: text}}
	}
	if len(inputs) == 0 {
		return nil
	}
	return s.enqueueQueueItem(
		surface,
		action.MessageID,
		action.Text,
		nil,
		inputs,
		"",
		workspaceKey,
		state.RouteModeNewThreadReady,
		surface.PromptOverride,
		false,
	)
}

func rewritePersonalDefaultWorkspaceNotices(events []eventcontract.Event) []eventcontract.Event {
	if len(events) == 0 {
		return events
	}
	rewritten := make([]eventcontract.Event, 0, len(events))
	addedNotice := false
	for i := range events {
		event := events[i]
		notice := event.Notice
		if notice == nil {
			rewritten = append(rewritten, event)
			continue
		}
		switch notice.Code {
		case "workspace_create_starting":
			if addedNotice {
				continue
			}
			notice.Code = "personal_workspace_starting"
			notice.Title = ""
			notice.Text = fmt.Sprintf("已进入个人聊天空间 %s，正在处理这条消息。", personalDefaultWorkspaceName)
		case "workspace_attached", "workspace_switched", "new_thread_ready":
			if addedNotice {
				continue
			}
			notice.Code = "personal_workspace_ready"
			notice.Title = ""
			notice.Text = fmt.Sprintf("已进入个人聊天空间 %s，直接发消息即可。", personalDefaultWorkspaceName)
		case "thread_selection_changed":
			continue
		default:
			rewritten = append(rewritten, event)
			continue
		}
		addedNotice = true
		event.Notice = notice
		if payload, ok := event.Payload.(eventcontract.NoticePayload); ok {
			payload.Notice = *notice
			event.Payload = payload
		}
		rewritten = append(rewritten, event)
	}
	return rewritten
}
