package eventcontract

import (
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/render"
)

type Payload interface {
	Kind() Kind
	isPayload()
}

type SnapshotPayload struct {
	Snapshot control.Snapshot
}

func (SnapshotPayload) Kind() Kind { return KindSnapshot }
func (SnapshotPayload) isPayload() {}

type SelectionPayload struct {
	View    control.FeishuSelectionView
	Context *control.FeishuUISelectionContext
}

func (SelectionPayload) Kind() Kind { return KindSelection }
func (SelectionPayload) isPayload() {}

type PagePayload struct {
	View    control.FeishuPageView
	Context *control.FeishuUIPageContext
}

func (PagePayload) Kind() Kind { return KindPage }
func (PagePayload) isPayload() {}

type RequestPayload struct {
	View    control.FeishuRequestView
	Context *control.FeishuUIRequestContext
}

func (RequestPayload) Kind() Kind { return KindRequest }
func (RequestPayload) isPayload() {}

type PathPickerPayload struct {
	View    control.FeishuPathPickerView
	Context *control.FeishuUIPathPickerContext
}

func (PathPickerPayload) Kind() Kind { return KindPathPicker }
func (PathPickerPayload) isPayload() {}

type TargetPickerPayload struct {
	View    control.FeishuTargetPickerView
	Context *control.FeishuUITargetPickerContext
}

func (TargetPickerPayload) Kind() Kind { return KindTargetPicker }
func (TargetPickerPayload) isPayload() {}

type ThreadHistoryPayload struct {
	View    control.FeishuThreadHistoryView
	Context *control.FeishuUIThreadHistoryContext
}

func (ThreadHistoryPayload) Kind() Kind { return KindThreadHistory }
func (ThreadHistoryPayload) isPayload() {}

type ProjectCockpitPayload struct {
	View control.ProjectCockpitView
}

func (ProjectCockpitPayload) Kind() Kind { return KindProjectCockpit }
func (ProjectCockpitPayload) isPayload() {}

type PendingInputPayload struct {
	State control.PendingInputState
}

func (PendingInputPayload) Kind() Kind { return KindPendingInput }
func (PendingInputPayload) isPayload() {}

type NoticePayload struct {
	Notice          control.Notice
	ThreadSelection *control.ThreadSelectionChanged
}

func (NoticePayload) Kind() Kind { return KindNotice }
func (NoticePayload) isPayload() {}

type PlanUpdatePayload struct {
	PlanUpdate control.PlanUpdate
}

func (PlanUpdatePayload) Kind() Kind { return KindPlanUpdate }
func (PlanUpdatePayload) isPayload() {}

type BlockCommittedPayload struct {
	Block             render.Block
	FileChangeSummary *control.FileChangeSummary
	TurnDiffSnapshot  *control.TurnDiffSnapshot
	TurnDiffPreview   *control.TurnDiffPreview
	FinalTurnSummary  *control.FinalTurnSummary
}

func (BlockCommittedPayload) Kind() Kind { return KindBlockCommitted }
func (BlockCommittedPayload) isPayload() {}

type TimelineTextPayload struct {
	TimelineText control.TimelineText
}

func (TimelineTextPayload) Kind() Kind { return KindTimelineText }
func (TimelineTextPayload) isPayload() {}

type ImageOutputPayload struct {
	ImageOutput control.ImageOutput
}

func (ImageOutputPayload) Kind() Kind { return KindImageOutput }
func (ImageOutputPayload) isPayload() {}

type ExecCommandProgressPayload struct {
	Progress control.ExecCommandProgress
}

func (ExecCommandProgressPayload) Kind() Kind { return KindExecCommandProgress }
func (ExecCommandProgressPayload) isPayload() {}

type AgentCommandPayload struct {
	Command agentproto.Command
}

func (AgentCommandPayload) Kind() Kind { return KindAgentCommand }
func (AgentCommandPayload) isPayload() {}

type DaemonCommandPayload struct {
	Command control.DaemonCommand
}

func (DaemonCommandPayload) Kind() Kind { return KindDaemonCommand }
func (DaemonCommandPayload) isPayload() {}

func PayloadKind(payload Payload) Kind {
	if payload == nil {
		return KindUnknown
	}
	return payload.Kind()
}
