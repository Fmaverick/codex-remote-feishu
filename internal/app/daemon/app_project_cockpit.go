package daemon

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
)

func (a *App) projectCockpitRefreshEventsLocked(instanceID string, event agentproto.Event, uiEvents []eventcontract.Event) []eventcontract.Event {
	if a == nil || a.service == nil {
		return nil
	}
	switch event.Kind {
	case agentproto.EventTurnStarted,
		agentproto.EventTurnCompleted,
		agentproto.EventItemStarted,
		agentproto.EventItemCompleted,
		agentproto.EventRequestStarted,
		agentproto.EventRequestResolved,
		agentproto.EventTurnPlanUpdated:
	default:
		return nil
	}
	for _, uiEvent := range uiEvents {
		if _, ok := projectCockpitPayloadFromEvent(uiEvent); ok {
			return nil
		}
	}
	for _, status := range a.service.ActiveRemoteTurns() {
		if strings.TrimSpace(status.InstanceID) != strings.TrimSpace(instanceID) {
			continue
		}
		if strings.TrimSpace(status.ThreadID) != "" && strings.TrimSpace(event.ThreadID) != "" && strings.TrimSpace(status.ThreadID) != strings.TrimSpace(event.ThreadID) {
			continue
		}
		if strings.TrimSpace(status.TurnID) != "" && strings.TrimSpace(event.TurnID) != "" && strings.TrimSpace(status.TurnID) != strings.TrimSpace(event.TurnID) {
			continue
		}
		return a.service.ProjectCockpitRefresh(status.SurfaceSessionID)
	}
	for _, status := range a.service.PendingRemoteTurns() {
		if strings.TrimSpace(status.InstanceID) == strings.TrimSpace(instanceID) {
			return a.service.ProjectCockpitRefresh(status.SurfaceSessionID)
		}
	}
	return nil
}
