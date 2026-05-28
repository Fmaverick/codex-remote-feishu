package daemon

import "github.com/kxn/codex-remote-feishu/internal/core/eventcontract"

func requestPayloadFromEvent(event eventcontract.Event) (eventcontract.RequestPayload, bool) {
	payload, ok := event.CanonicalPayload().(eventcontract.RequestPayload)
	return payload, ok
}

func pagePayloadFromEvent(event eventcontract.Event) (eventcontract.PagePayload, bool) {
	payload, ok := event.CanonicalPayload().(eventcontract.PagePayload)
	return payload, ok
}

func threadHistoryPayloadFromEvent(event eventcontract.Event) (eventcontract.ThreadHistoryPayload, bool) {
	payload, ok := event.CanonicalPayload().(eventcontract.ThreadHistoryPayload)
	return payload, ok
}

func projectCockpitPayloadFromEvent(event eventcontract.Event) (eventcontract.ProjectCockpitPayload, bool) {
	payload, ok := event.CanonicalPayload().(eventcontract.ProjectCockpitPayload)
	return payload, ok
}

func targetPickerPayloadFromEvent(event eventcontract.Event) (eventcontract.TargetPickerPayload, bool) {
	payload, ok := event.CanonicalPayload().(eventcontract.TargetPickerPayload)
	return payload, ok
}

func pathPickerPayloadFromEvent(event eventcontract.Event) (eventcontract.PathPickerPayload, bool) {
	payload, ok := event.CanonicalPayload().(eventcontract.PathPickerPayload)
	return payload, ok
}

func noticePayloadFromEvent(event eventcontract.Event) (eventcontract.NoticePayload, bool) {
	payload, ok := event.CanonicalPayload().(eventcontract.NoticePayload)
	return payload, ok
}

func blockPayloadFromEvent(event eventcontract.Event) (eventcontract.BlockCommittedPayload, bool) {
	payload, ok := event.CanonicalPayload().(eventcontract.BlockCommittedPayload)
	return payload, ok
}

func execCommandProgressPayloadFromEvent(event eventcontract.Event) (eventcontract.ExecCommandProgressPayload, bool) {
	payload, ok := event.CanonicalPayload().(eventcontract.ExecCommandProgressPayload)
	return payload, ok
}
