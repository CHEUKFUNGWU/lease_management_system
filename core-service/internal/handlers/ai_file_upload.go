package handlers

// W5-5: the file upload endpoint moves from ai-service to core-service. It
// validates the content type, caps the size, stores the object in MinIO and
// returns the same response shape the front-end /api/ai/files/upload consumes.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var allowedUploadTypes = map[string]string{
	"application/pdf": ".pdf",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": ".xlsx",
	"application/vnd.ms-excel": ".xls",
	"image/jpeg":               ".jpg",
	"image/png":                ".png",
	"image/tiff":               ".tiff",
}

var allowedUploadTaskTypes = map[string]bool{"contract": true, "payment_schedule": true, "event": true, "scan_copy": true}

const maxUploadBytes = 50 * 1024 * 1024

// AIFileUploader is the upload seam behind the handler.
type AIFileUploader interface {
	PutObject(ctx context.Context, objectName string, data []byte, contentType string) (string, error)
}

// UploadAIFile handles POST /api/v1/ai/files/upload.
func UploadAIFile(uploader AIFileUploader) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := c.Request.ParseMultipartForm(maxUploadBytes + 1<<20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "multipart body required"})
			return
		}
		fileHeader, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file part required"})
			return
		}
		contentType := fileHeader.Header.Get("Content-Type")
		ext, ok := allowedUploadTypes[contentType]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的文件类型: " + contentType + ". 支持: PDF, Excel, JPG, PNG, TIFF"})
			return
		}
		taskType := c.PostForm("task_type")
		if taskType == "" {
			taskType = "contract"
		}
		if !allowedUploadTaskTypes[taskType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的任务类型: " + taskType + ". 支持: contract, payment_schedule, event, scan_copy"})
			return
		}
		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "open upload"})
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read upload"})
			return
		}
		if len(data) > maxUploadBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "文件大小超过 50MB 限制"})
			return
		}

		fileID := uuid.NewString()
		now := time.Now()
		objectName := fmt.Sprintf("%s/%04d/%02d/%s%s", taskType, now.Year(), now.Month(), fileID, ext)
		fileURL, err := uploader.PutObject(c.Request.Context(), objectName, data, contentType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "上传失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"file_id":       fileID,
			"original_name": fileHeader.Filename,
			"object_name":   objectName,
			"file_url":      fileURL,
			"file_size":     len(data),
			"content_type":  contentType,
			"task_type":     taskType,
			"uploaded_at":   now.Format(time.RFC3339),
		})
	}
}

var _ = strings.TrimSpace
