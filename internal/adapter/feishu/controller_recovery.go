package feishu

import (
	"errors"
	"log"
	"strings"
	"time"
)

var gatewayRecoveryBackoff = []time.Duration{
	5 * time.Second,
	15 * time.Second,
	30 * time.Second,
	60 * time.Second,
	2 * time.Minute,
	5 * time.Minute,
}

func (c *MultiGatewayController) maybeScheduleGatewayRecoveryLocked(gatewayID string, worker *gatewayWorker, generation uint64, state GatewayState, err error) {
	if worker == nil || !gatewayStateRecoverable(state, err) {
		return
	}
	if !worker.config.Enabled {
		log.Printf("feishu gateway auto-reconnect skipped: gateway=%s reason=disabled state=%s", gatewayID, state)
		return
	}
	if !workerHasCredentials(worker.config) {
		log.Printf("feishu gateway auto-reconnect skipped: gateway=%s reason=missing_credentials state=%s", gatewayID, state)
		return
	}
	if c.startCtx == nil || c.startCtx.Err() != nil {
		log.Printf("feishu gateway auto-reconnect skipped: gateway=%s reason=controller_stopped state=%s", gatewayID, state)
		return
	}
	if worker.recoveryTimer != nil && worker.recoveryGeneration == generation {
		log.Printf("feishu gateway auto-reconnect skipped: gateway=%s reason=already_scheduled generation=%d state=%s", gatewayID, generation, state)
		return
	}

	delay := gatewayRecoveryDelay(worker.recoveryAttempt)
	worker.recoveryAttempt++
	worker.recoveryGeneration = generation
	if worker.recoveryTimer != nil {
		worker.recoveryTimer.Stop()
	}
	log.Printf("feishu gateway auto-reconnect scheduled: gateway=%s generation=%d attempt=%d delay=%s state=%s err=%v", gatewayID, generation, worker.recoveryAttempt, delay, state, err)
	worker.recoveryTimer = time.AfterFunc(delay, func() {
		c.runScheduledGatewayRecovery(gatewayID, generation)
	})
}

func (c *MultiGatewayController) runScheduledGatewayRecovery(gatewayID string, generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	worker := c.workers[gatewayID]
	if worker == nil {
		log.Printf("feishu gateway auto-reconnect skipped: gateway=%s reason=worker_removed generation=%d", gatewayID, generation)
		return
	}
	if worker.generation != generation || worker.recoveryGeneration != generation {
		log.Printf("feishu gateway auto-reconnect stale generation ignored: gateway=%s scheduled_generation=%d current_generation=%d recovery_generation=%d", gatewayID, generation, worker.generation, worker.recoveryGeneration)
		return
	}
	worker.recoveryTimer = nil
	worker.recoveryGeneration = 0
	if !worker.config.Enabled {
		log.Printf("feishu gateway auto-reconnect skipped: gateway=%s reason=disabled generation=%d", gatewayID, generation)
		return
	}
	if !workerHasCredentials(worker.config) {
		log.Printf("feishu gateway auto-reconnect skipped: gateway=%s reason=missing_credentials generation=%d", gatewayID, generation)
		return
	}
	if c.startCtx == nil || c.startCtx.Err() != nil {
		log.Printf("feishu gateway auto-reconnect skipped: gateway=%s reason=controller_stopped generation=%d", gatewayID, generation)
		return
	}

	log.Printf("feishu gateway auto-reconnect started: gateway=%s previous_generation=%d attempt=%d", gatewayID, generation, worker.recoveryAttempt)
	c.stopWorkerLocked(worker)
	if err := c.ensureWorkerRunningLocked(gatewayID); err != nil {
		worker.status.State = GatewayStateDegraded
		worker.status.LastError = err.Error()
		log.Printf("feishu gateway auto-reconnect failed: gateway=%s err=%v", gatewayID, err)
		c.maybeScheduleGatewayRecoveryLocked(gatewayID, worker, worker.generation, GatewayStateDegraded, err)
	}
}

func gatewayRecoveryDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt >= len(gatewayRecoveryBackoff) {
		return gatewayRecoveryBackoff[len(gatewayRecoveryBackoff)-1]
	}
	return gatewayRecoveryBackoff[attempt]
}

func gatewayStateRecoverable(state GatewayState, err error) bool {
	switch state {
	case GatewayStateDegraded:
		return gatewayErrorRecoverable(err)
	case GatewayStateStopped:
		return true
	default:
		return false
	}
}

func gatewayErrorRecoverable(err error) bool {
	if err == nil {
		return true
	}
	var runnerErr *gatewayRunnerError
	if errors.As(err, &runnerErr) {
		switch runnerErr.Code() {
		case "connect_failed":
			return true
		case "auth_failed":
			return strings.Contains(strings.ToLower(runnerErr.Error()), "ticket is invalid")
		default:
			return false
		}
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "ticket is invalid") {
		return true
	}
	if strings.Contains(text, "invalid access token") {
		return true
	}
	return strings.Contains(text, "connect_failed")
}
