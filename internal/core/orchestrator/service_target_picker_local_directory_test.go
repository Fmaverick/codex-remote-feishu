package orchestrator

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/testutil"
)

func TestTargetPickerConfirmAddWorkspaceLocalDirectoryIgnoresReplayedEmptyDraftOnSecondConfirm(t *testing.T) {
	now := time.Date(2026, 4, 14, 15, 49, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	workspaceRoot := t.TempDir()

	view := singleTargetPickerEvent(t, svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionListInstances,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
	}))
	addMode := openAddWorkspaceLocalDirectoryPage(t, svc, view)
	surface := svc.root.Surfaces["surface-1"]
	record := svc.activeTargetPicker(surface)
	record.LocalDirectoryPath = workspaceRoot

	checkEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionTargetPickerConfirm,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		PickerID:         addMode.PickerID,
	})
	if len(checkEvents) != 1 || checkEvents[0].TargetPickerView == nil {
		t.Fatalf("expected same-card checked state before replayed confirm, got %#v", checkEvents)
	}
	if got := checkEvents[0].TargetPickerView; !got.LocalDirectoryChecked || got.ConfirmLabel != "接入并继续" {
		t.Fatalf("expected first confirm to produce checked local-directory state, got %#v", got)
	}

	confirmEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionTargetPickerConfirm,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		PickerID:         addMode.PickerID,
		RequestAnswers: map[string][]string{
			control.FeishuTargetPickerLocalDirectoryNameFieldName: {""},
		},
	})
	if surface.PendingHeadless == nil || !surface.PendingHeadless.PrepareNewThread || !testutil.SamePath(surface.PendingHeadless.ThreadCWD, workspaceRoot) {
		t.Fatalf("expected replayed empty local-directory draft to keep validated state and start headless, got %#v", surface.PendingHeadless)
	}
	if len(confirmEvents) == 0 || confirmEvents[0].TargetPickerView == nil {
		t.Fatalf("expected processing card after replayed confirm, got %#v", confirmEvents)
	}
	if got := confirmEvents[0].TargetPickerView; got.Stage != control.FeishuTargetPickerStageProcessing || got.StatusTitle != "正在接入工作区" {
		t.Fatalf("expected replayed confirm to continue into processing, got %#v", got)
	}
}

func TestTargetPickerConfirmAddWorkspaceLocalDirectoryChangingNameInvalidatesPreviousCheck(t *testing.T) {
	now := time.Date(2026, 4, 14, 15, 49, 30, 0, time.UTC)
	svc := newServiceForTest(&now)
	workspaceRoot := t.TempDir()

	view := singleTargetPickerEvent(t, svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionListInstances,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
	}))
	addMode := openAddWorkspaceLocalDirectoryPage(t, svc, view)
	surface := svc.root.Surfaces["surface-1"]
	record := svc.activeTargetPicker(surface)
	record.LocalDirectoryPath = workspaceRoot

	checkEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionTargetPickerConfirm,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		PickerID:         addMode.PickerID,
	})
	if len(checkEvents) != 1 || checkEvents[0].TargetPickerView == nil {
		t.Fatalf("expected checked state before changing directory name, got %#v", checkEvents)
	}
	if got := checkEvents[0].TargetPickerView; !got.LocalDirectoryChecked || got.ConfirmLabel != "接入并继续" {
		t.Fatalf("expected first confirm to check original local-directory draft, got %#v", got)
	}

	renameEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionTargetPickerConfirm,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		PickerID:         addMode.PickerID,
		RequestAnswers: map[string][]string{
			control.FeishuTargetPickerLocalDirectoryNameFieldName: {"child"},
		},
	})
	if surface.PendingHeadless != nil {
		t.Fatalf("expected changed local-directory draft to require a fresh check before headless start, got %#v", surface.PendingHeadless)
	}
	if len(renameEvents) != 1 || renameEvents[0].TargetPickerView == nil {
		t.Fatalf("expected changed local-directory draft to stay on owner card, got %#v", renameEvents)
	}
	got := renameEvents[0].TargetPickerView
	expectedFinalPath := filepath.Join(got.LocalDirectoryPath, "child")
	if !got.LocalDirectoryChecked || got.ConfirmLabel != "创建并继续" || !got.CanConfirm || got.LocalDirectoryName != "child" || !testutil.SamePath(got.LocalDirectoryFinalPath, expectedFinalPath) {
		t.Fatalf("expected changed local-directory draft to be rechecked as the new target, got %#v", got)
	}
}
