package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/agentartifact"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

const (
	exportContentTypeXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	exportContentTypeDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
)

// LintRejected reports that a working paper failed the fail-closed lint and
// therefore must not be exported.
type LintRejected struct {
	Violations []workingpaper.Violation
}

func (e *LintRejected) Error() string {
	return fmt.Sprintf("working paper failed lint: %d violation(s)", len(e.Violations))
}

// ExportArtifact renders a working_paper artifact to xlsx or docx. The
// workingpaper.Lint gate runs first and fails closed (design decision D-B):
// a paper violating any invariant is refused with its violations, never
// best-effort exported.
func (h *AIChatHandler) ExportArtifact(c *gin.Context) {
	artifactID := c.Param("id")
	format := c.Query("format")

	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDStr, _ := userID.(string)
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}

	artifact, err := h.runtimeRepo.GetArtifactByID(c.Request.Context(), artifactID, userIDStr, entity)
	if err != nil {
		writeRunAccessError(c, err)
		return
	}

	out, contentType, err := RenderArtifactExport(artifact.Data, agentartifact.ArtifactType(artifact.ArtifactType), format, time.Now())
	if err != nil {
		var lintErr *LintRejected
		if errors.As(err, &lintErr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": lintErr.Error(), "violations": lintErr.Violations})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filename := artifactID + ".xlsx"
	if format == "docx" {
		filename = artifactID + ".docx"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(http.StatusOK, contentType, out)
}

// RenderArtifactExport is the seam the tests cross: artifact data in, file
// bytes out, with the lint gate inside. The audit cross-check (invariant I2's
// database half) is wired by callers with real audit access; the structural
// half (tool_call_id presence) is always enforced.
func RenderArtifactExport(data json.RawMessage, artifactType agentartifact.ArtifactType, format string, now time.Time) ([]byte, string, error) {
	if artifactType != agentartifact.ArtifactWorkingPaper {
		return nil, "", errors.New("artifact is not a working paper")
	}
	var paper workingpaper.Paper
	if err := json.Unmarshal(data, &paper); err != nil {
		return nil, "", fmt.Errorf("artifact data is not a working paper: %w", err)
	}
	paper = workingpaper.Build(paper, now)
	rep := workingpaper.Lint(paper, nil)
	if !rep.OK {
		return nil, "", &LintRejected{Violations: rep.Violations}
	}
	switch format {
	case "xlsx":
		out, err := workingpaper.RenderXLSX(paper, now)
		return out, exportContentTypeXLSX, err
	case "docx":
		out, err := workingpaper.RenderDOCX(paper, now)
		return out, exportContentTypeDOCX, err
	default:
		return nil, "", errors.New("format must be xlsx or docx")
	}
}
