package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
	"github.com/kxn/codex-remote-feishu/internal/testutil"
)

func TestPersonalPrivateTextCreatesDefaultWorkspaceAndQueuesFirstMessage(t *testing.T) {
	now := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	home := t.TempDir()
	t.Setenv("HOME", home)
	workspaceRoot := filepath.Join(home, personalDefaultWorkspaceName)

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionTextMessage,
		GatewayID:        "app-1",
		SurfaceSessionID: "feishu:app-1:user:ou-user",
		ChatID:           "oc-private",
		ActorUserID:      "ou-user",
		MessageID:        "msg-1",
		Text:             "你好",
	})
	if _, err := os.Stat(workspaceRoot); err != nil {
		t.Fatalf("expected personal workspace directory to be created: %v", err)
	}
	surface := svc.Surface("feishu:app-1:user:ou-user")
	if surface == nil || surface.PendingHeadless == nil {
		t.Fatalf("expected pending personal headless launch, got %#v", surface)
	}
	pending := surface.PendingHeadless
	if pending.Purpose != state.HeadlessLaunchPurposeFreshWorkspace || !pending.PrepareNewThread || !testutil.SamePath(pending.WorkspaceKey, workspaceRoot) {
		t.Fatalf("unexpected pending launch: %#v", pending)
	}
	if len(surface.QueuedQueueItemIDs) != 0 {
		t.Fatalf("expected first text to wait for headless attach before queueing, got %#v", surface.QueuedQueueItemIDs)
	}
	if stored, ok := svc.personalDefaultInputs[pending.InstanceID]; !ok || stored.Text != "你好" {
		t.Fatalf("expected first text to be staged for pending headless launch, got %#v", svc.personalDefaultInputs)
	}
	if !hasNoticeCode(events, "personal_workspace_starting") || !hasDaemonCommand(events, control.DaemonCommandStartHeadless) {
		t.Fatalf("expected light personal notice and headless start command, got %#v", events)
	}

	svc.UpsertInstance(&state.InstanceRecord{
		InstanceID:    pending.InstanceID,
		DisplayName:   personalDefaultWorkspaceName,
		WorkspaceRoot: workspaceRoot,
		WorkspaceKey:  workspaceRoot,
		Source:        "headless",
		Managed:       true,
		Online:        true,
		Threads:       map[string]*state.ThreadRecord{},
	})
	connectEvents := svc.ApplyInstanceConnected(pending.InstanceID)
	if !hasPromptSend(connectEvents) {
		t.Fatalf("expected queued first message to dispatch after headless connects, got %#v", connectEvents)
	}
	snapshot := svc.SurfaceSnapshot("feishu:app-1:user:ou-user")
	if snapshot == nil || snapshot.Attachment.InstanceID != pending.InstanceID || snapshot.PendingHeadless.InstanceID != "" || !testutil.SamePath(snapshot.WorkspaceKey, workspaceRoot) {
		t.Fatalf("expected personal workspace attached after connect, got %#v", snapshot)
	}
}

func TestPersonalDefaultWorkspaceDoesNotApplyToGroupChat(t *testing.T) {
	now := time.Date(2026, 5, 24, 10, 5, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	home := t.TempDir()
	t.Setenv("HOME", home)

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionTextMessage,
		GatewayID:        "app-1",
		SurfaceSessionID: "feishu:app-1:chat:oc-group",
		ChatID:           "oc-group",
		ActorUserID:      "ou-user",
		MessageID:        "msg-1",
		Text:             "你好",
	})
	if !hasNoticeCode(events, "not_attached") {
		t.Fatalf("expected group chat to keep detached flow, got %#v", events)
	}
	if _, err := os.Stat(filepath.Join(home, personalDefaultWorkspaceName)); !os.IsNotExist(err) {
		t.Fatalf("expected group chat not to create personal workspace, stat err=%v", err)
	}
}

func TestPersonalDefaultWorkspaceDoesNotApplyToVSCodeMode(t *testing.T) {
	now := time.Date(2026, 5, 24, 10, 10, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	home := t.TempDir()
	t.Setenv("HOME", home)
	svc.MaterializeSurfaceResumeContract("feishu:app-1:user:ou-user", "app-1", "oc-private", "ou-user", state.VSCodeSurfaceBackendContract(), "", "")

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionTextMessage,
		GatewayID:        "app-1",
		SurfaceSessionID: "feishu:app-1:user:ou-user",
		ChatID:           "oc-private",
		ActorUserID:      "ou-user",
		MessageID:        "msg-1",
		Text:             "你好",
	})
	if !hasNoticeCode(events, "not_attached") {
		t.Fatalf("expected vscode mode to keep detached flow, got %#v", events)
	}
	if _, err := os.Stat(filepath.Join(home, personalDefaultWorkspaceName)); !os.IsNotExist(err) {
		t.Fatalf("expected vscode mode not to create personal workspace, stat err=%v", err)
	}
}

func TestWhereReportsCompactPersonalStatus(t *testing.T) {
	now := time.Date(2026, 5, 24, 10, 15, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	svc.MaterializeSurface("surface-1", "app-1", "chat-1", "user-1")
	surface := svc.Surface("surface-1")
	surface.ClaimedWorkspaceKey = "/Users/test/CodexChat"
	surface.RouteMode = state.RouteModeNewThreadReady
	surface.PreparedThreadCWD = "/Users/test/CodexChat"

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionWhere,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
	})
	if len(events) != 1 || events[0].Notice == nil || events[0].Notice.Code != "where" {
		t.Fatalf("expected one where notice, got %#v", events)
	}
	wantParts := []string{"当前：CodexChat", "新会话待命", "codex", "default"}
	for _, part := range wantParts {
		if !strings.Contains(events[0].Notice.Text, part) {
			t.Fatalf("where text %q missing %q", events[0].Notice.Text, part)
		}
	}
	if events[0].PageView != nil || events[0].Snapshot != nil {
		t.Fatalf("expected lightweight notice without card payload, got %#v", events[0])
	}
}

func hasNoticeCode(events []eventcontract.Event, code string) bool {
	for _, event := range events {
		if event.Notice != nil && event.Notice.Code == code {
			return true
		}
	}
	return false
}

func hasDaemonCommand(events []eventcontract.Event, kind control.DaemonCommandKind) bool {
	for _, event := range events {
		if event.DaemonCommand != nil && event.DaemonCommand.Kind == kind {
			return true
		}
	}
	return false
}

func hasPromptSend(events []eventcontract.Event) bool {
	for _, event := range events {
		if event.Command != nil && event.Command.Kind == agentproto.CommandPromptSend {
			return true
		}
	}
	return false
}
