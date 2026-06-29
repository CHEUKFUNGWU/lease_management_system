package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
)

type Logger struct {
	repo *repository.AuditRepository
}

func NewLogger(repo *repository.AuditRepository) *Logger {
	return &Logger{repo: repo}
}

// WithTx returns a logger that writes through the given transaction, so an audit
// record commits atomically with the change it describes.
func (l *Logger) WithTx(tx repository.DBTX) *Logger {
	return &Logger{repo: l.repo.WithTx(tx)}
}

// Metadata carries the request-scoped actor and origin of a change, decoupled
// from any HTTP framework so services can audit without importing gin.
type Metadata struct {
	ChangedBy string
	IPAddress string
	UserAgent string
}

// MetadataFromGin extracts audit metadata from a gin request context.
func MetadataFromGin(changedBy string, c *gin.Context) Metadata {
	m := Metadata{ChangedBy: changedBy}
	if c != nil {
		m.IPAddress = c.ClientIP()
		m.UserAgent = c.Request.UserAgent()
	}
	return m
}

// Log records an audit entry, taking the actor and request origin from gin.
func (l *Logger) Log(ctx context.Context, tableName, recordID, action string, oldVals, newVals interface{}, changedBy string, c *gin.Context) error {
	return l.LogEvent(ctx, tableName, recordID, action, oldVals, newVals, MetadataFromGin(changedBy, c))
}

// LogEvent records an audit entry from framework-independent metadata. It is the
// core path used by both the gin-based Log helper and by services that audit
// inside a transaction.
func (l *Logger) LogEvent(ctx context.Context, tableName, recordID, action string, oldVals, newVals interface{}, meta Metadata) error {
	log := &repository.AuditLog{
		TableName: tableName,
		RecordID:  recordID,
		Action:    action,
		OldValues: marshalJSON(oldVals),
		NewValues: marshalJSON(newVals),
		ChangedBy: strPtrOrNil(meta.ChangedBy),
		ChangedAt: time.Now(),
		IPAddress: strPtrOrNil(meta.IPAddress),
		UserAgent: strPtrOrNil(meta.UserAgent),
	}
	return l.repo.Create(ctx, log)
}

func marshalJSON(v interface{}) *string {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	s := string(b)
	return &s
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
