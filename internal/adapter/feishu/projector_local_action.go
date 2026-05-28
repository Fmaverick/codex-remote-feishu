package feishu

import (
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/frontstagecontract"
)

func localCardActionPayload(actionKind control.ActionKind, actionArg, daemonLifecycleID string) map[string]any {
	return frontstagecontract.ActionPayloadWithLifecycle(frontstagecontract.ActionPayloadPageAction(string(actionKind), actionArg), daemonLifecycleID)
}
