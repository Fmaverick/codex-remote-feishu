package orchestrator

import (
	"fmt"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func (s *Service) where(surface *state.SurfaceConsoleRecord) []eventcontract.Event {
	return notice(surface, "where", fmt.Sprintf("当前：%s · %s · %s · %s",
		s.whereWorkspaceLabel(surface),
		s.whereRouteLabel(surface),
		s.whereBackendLabel(surface),
		s.whereProfileLabel(surface),
	))
}

func (s *Service) whereWorkspaceLabel(surface *state.SurfaceConsoleRecord) string {
	if surface == nil {
		return "未进入"
	}
	if workspaceKey := s.surfaceCurrentWorkspaceKey(surface); workspaceKey != "" {
		if name := state.WorkspaceShortName(workspaceKey); name != "" {
			return name
		}
		return workspaceKey
	}
	if inst := s.root.Instances[strings.TrimSpace(surface.AttachedInstanceID)]; inst != nil {
		if name := state.WorkspaceShortName(firstNonEmpty(inst.WorkspaceKey, inst.WorkspaceRoot)); name != "" {
			return name
		}
		if strings.TrimSpace(inst.DisplayName) != "" {
			return strings.TrimSpace(inst.DisplayName)
		}
	}
	return "未进入"
}

func (s *Service) whereRouteLabel(surface *state.SurfaceConsoleRecord) string {
	if surface == nil {
		return "未连接"
	}
	if surface.PendingHeadless != nil {
		return "正在进入"
	}
	switch surface.RouteMode {
	case state.RouteModeNewThreadReady:
		return "新会话待命"
	case state.RouteModeFollowLocal:
		if strings.TrimSpace(surface.SelectedThreadID) != "" {
			return "跟随当前会话"
		}
		return "等待 VS Code 会话"
	case state.RouteModePinned:
		if strings.TrimSpace(surface.SelectedThreadID) != "" {
			return "继续当前会话"
		}
	}
	if strings.TrimSpace(surface.AttachedInstanceID) != "" {
		return "未选择会话"
	}
	return "未连接"
}

func (s *Service) whereBackendLabel(surface *state.SurfaceConsoleRecord) string {
	backend := s.surfaceBackend(surface)
	if backend == "" {
		backend = agentproto.BackendCodex
	}
	return string(backend)
}

func (s *Service) whereProfileLabel(surface *state.SurfaceConsoleRecord) string {
	switch s.surfaceBackend(surface) {
	case agentproto.BackendClaude:
		if profileID := strings.TrimSpace(s.surfaceClaudeProfileID(surface)); profileID != "" {
			return profileID
		}
	default:
		if providerID := strings.TrimSpace(s.surfaceCodexProviderID(surface)); providerID != "" {
			return providerID
		}
	}
	return "default"
}
