package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type adminSurfaceWorkspaceAttachRequest struct {
	WorkspaceRoot    string `json:"workspaceRoot"`
	PrepareNewThread *bool  `json:"prepareNewThread,omitempty"`
}

type adminSurfaceWorkspaceAttachResponse struct {
	SurfaceSessionID string `json:"surfaceSessionId"`
	WorkspaceRoot    string `json:"workspaceRoot"`
	PrepareNewThread bool   `json:"prepareNewThread"`
	EventCount       int    `json:"eventCount"`
}

func (a *App) handleAdminSurfaceWorkspaceAttach(w http.ResponseWriter, r *http.Request) {
	surfaceID := strings.TrimSpace(r.PathValue("surface"))
	if surfaceID == "" {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    "surface_required",
			Message: "surface is required",
		})
		return
	}

	var req adminSurfaceWorkspaceAttachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    "invalid_json",
			Message: "request body must be valid JSON",
		})
		return
	}
	workspaceRoot, err := normalizeWorkspaceRoot(req.WorkspaceRoot)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiError{
			Code:    "invalid_workspace_root",
			Message: err.Error(),
		})
		return
	}
	prepareNewThread := true
	if req.PrepareNewThread != nil {
		prepareNewThread = *req.PrepareNewThread
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.service.Surface(surfaceID) == nil {
		writeAPIError(w, http.StatusNotFound, apiError{
			Code:    "surface_not_found",
			Message: "surface was not found; send any message to the Feishu bot first, then retry",
		})
		return
	}
	events := a.service.AttachWorkspaceForSurface(surfaceID, workspaceRoot, prepareNewThread)
	a.handleUIEventsLocked(context.Background(), events)
	a.syncSurfaceResumeStateForSurfacesLocked([]string{surfaceID}, nil)
	a.syncClaudeWorkspaceProfileStateLocked()
	a.syncWorkspaceSurfaceContextFilesLocked()

	writeJSON(w, http.StatusOK, adminSurfaceWorkspaceAttachResponse{
		SurfaceSessionID: surfaceID,
		WorkspaceRoot:    workspaceRoot,
		PrepareNewThread: prepareNewThread,
		EventCount:       len(events),
	})
}
