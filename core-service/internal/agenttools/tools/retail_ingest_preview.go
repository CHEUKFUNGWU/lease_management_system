package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/pagefill"
	"github.com/lease-management-system/core-service/internal/services/retailingest"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

// RetailIngestPreviewArguments is the strict input of the agent-side import
// preview. The file arrives by reference: the reader seam resolves bytes.
type RetailIngestPreviewArguments struct {
	FileID       string `json:"file_id"`
	ObjectName   string `json:"object_name"`
	ContentType  string `json:"content_type"`
	SourceSystem string `json:"source_system"`
	AsOf         string `json:"as_of,omitempty"`
}

// IngestFileReader resolves uploaded file bytes. Production wiring lands with
// W5 (minio-go in core-service); until then the tool refuses honestly when
// the seam is absent — never fabricates a preview.
type IngestFileReader interface {
	ReadObject(ctx context.Context, objectName string) ([]byte, error)
}

// NewRetailIngestPreviewDefinition registers the agent-side fill tool for the
// retail import page (appendix A.4). LevelDraft, skill-scoped to
// retail_ingest_fill; the output is a page_fill payload whose payload region
// contains only the human-provided envelope fields — mapping suggestions stay
// in the suggestions region with rule provenance until confirmed.
func NewRetailIngestPreviewDefinition(reader IngestFileReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "retail.store_days.import.preview",
			Version:     "v1",
			DisplayName: "零售导入预填预览",
			Description: "解析经营数据文件并为零售数据导入页生成预填：source_system/as_of 由人提供，列映射以建议形式呈现，确认后才能入库。Agent 无 commit 权限——commit 永远是人。",
			Level:       agenttools.LevelDraft,
			ReadOnly:    false,
			Permissions: []agenttools.Permission{{Resource: "master_data", Action: "manage"}},
			InputSchema: json.RawMessage(`{
				"type": "object",
				"required": ["file_id", "object_name", "content_type", "source_system"],
				"properties": {
					"file_id": {"type": "string"},
					"object_name": {"type": "string"},
					"content_type": {"type": "string"},
					"source_system": {"type": "string"},
					"as_of": {"type": "string", "pattern": "^[0-9]{4}-[0-9]{2}-[0-9]{2}$"}
				}
			}`),
			OutputSchema:        json.RawMessage(`{"type": "object", "required": ["page_fill", "side_effects"]}`),
			Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"mapping_unconfirmed", "import_review"}, ConfirmAction: "confirm"},
			SupportsDryRun:      true,
			SupportsIdempotency: true,
			MaxRows:             1000,
			TimeoutSeconds:      60,
		},
		SkillIDs: []string{ecommerceSkill}, // agent-universal-pagefill-v1 P0-A①：同零售运营技能族（旧值指向从未注册的 retail_ingest_fill）
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return agenttools.ToolResult{}, err
			}
			var args RetailIngestPreviewArguments
			dec := json.NewDecoder(strings.NewReader(string(call.Arguments)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&args); err != nil {
				return agenttools.ToolResult{}, errors.New("invalid retail ingest preview arguments")
			}
			if strings.TrimSpace(args.SourceSystem) == "" {
				return agenttools.ToolResult{}, errors.New("source_system is required")
			}
			if reader == nil {
				return agenttools.ToolResult{}, errors.New("ingest file reader is not wired (W5 minio-go); no preview can be produced")
			}

			raw, err := reader.ReadObject(ctx, args.ObjectName)
			if err != nil {
				return agenttools.ToolResult{}, fmt.Errorf("read uploaded file: %w", err)
			}
			format := retailingest.Format(formatFromName(args.ObjectName))
			headers, rows, err := retailingest.ParseTemplate(raw, format)
			if err != nil {
				return agenttools.ToolResult{}, fmt.Errorf("parse template: %w", err)
			}
			profiles := retailingest.ColumnProfiles(headers, rows)
			// The deterministic rule table is the suggestion source here; the
			// AI-assisted overlay lives at handler wiring, not in the tool.
			mapping := retailingest.SuggestMapping(headers, profiles)

			fill := pagefill.New(
				"retail-data-import",
				"POST /retail/operating-facts/store-days/import/preview",
				"/retail-data-import?fill="+call.CallID,
			)
			if err := fill.PutPayload("source_system", args.SourceSystem, workingpaper.Provenance{
				Basis: workingpaper.BasisHumanInput, ConfirmedBy: execution.Principal.UserID, ConfirmedAt: time.Now().UTC().Format(time.RFC3339),
			}); err != nil {
				return agenttools.ToolResult{}, err
			}
			if args.AsOf != "" {
				if err := fill.PutPayload("as_of", args.AsOf, workingpaper.Provenance{
					Basis: workingpaper.BasisHumanInput, ConfirmedBy: execution.Principal.UserID, ConfirmedAt: time.Now().UTC().Format(time.RFC3339),
				}); err != nil {
					return agenttools.ToolResult{}, err
				}
			}
			// Unconfirmed mapping suggestions stay Exploratory — structurally
			// barred from the payload, confirmed only by the human on the page.
			fill.Suggest("mapping", mapping, workingpaper.Provenance{
				Basis:         workingpaper.BasisExploratory,
				EngineVersion: "rule-mapping-v1",
			})
			if err := fill.Validate(); err != nil {
				return agenttools.ToolResult{}, err
			}

			return agenttools.ToolResult{
				CallID: call.CallID,
				Status: agenttools.StatusCompleted,
				Data: map[string]any{
					"page_fill":    fill,
					"side_effects": false,
				},
				Review: agenttools.ReviewResult{Required: true, Reasons: []string{"mapping_unconfirmed"}},
			}, nil
		},
	}
}

func formatFromName(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".csv"):
		return "csv"
	case strings.HasSuffix(lower, ".xlsx"):
		return "xlsx"
	case strings.HasSuffix(lower, ".xls"):
		return "xls"
	case strings.HasSuffix(lower, ".tsv"):
		return "csv"
	default:
		return "xlsx"
	}
}
