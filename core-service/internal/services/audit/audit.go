package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/repository"
)

type Logger struct {
	repo *repository.AuditRepository
}

func NewLogger(repo *repository.AuditRepository) *Logger {
	return &Logger{repo: repo}
}

func (l *Logger) Log(ctx context.Context, tableName, recordID, action string, oldVals, newVals interface{}, changedBy string, c *gin.Context) error {
	var oldJSON, newJSON *string
	if oldVals != nil {
		b, _ := json.Marshal(oldVals)
		s := string(b)
		oldJSON = &s
	}
	if newVals != nil {
		b, _ := json.Marshal(newVals)
		s := string(b)
		newJSON = &s
	}

	var ipAddr, userAgent *string
	if c != nil {
		if ip := c.ClientIP(); ip != "" {
			ipAddr = &ip
		}
		if ua := c.Request.UserAgent(); ua != "" {
			userAgent = &ua
		}
	}

	var cb *string
	if changedBy != "" {
		cb = &changedBy
	}

	log := &repository.AuditLog{
		TableName: tableName,
		RecordID:  recordID,
		Action:    action,
		OldValues: oldJSON,
		NewValues: newJSON,
		ChangedBy: cb,
		ChangedAt: time.Now(),
		IPAddress: ipAddr,
		UserAgent: userAgent,
	}

	return l.repo.Create(ctx, log)
}
