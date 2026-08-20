package aiagent

// W5-3 wiring: the four parse endpoints now run the intake producer in-process.
// A file is downloaded from MinIO, decoded to text/deterministic records, the
// in-process LLM (internal/llm) is called with the pinned prompt, and the
// normalized draft envelope is decoded back through the same ai-intake.v1
// consumption seam — the downstream contract and review-gate semantics are
// unchanged.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lease-management-system/core-service/internal/aiintake"
	"github.com/lease-management-system/core-service/internal/docparse"
	"github.com/lease-management-system/core-service/internal/llm"
)

// llmCompleter adapts the in-process LLM client to the producer's completer
// seam (system+prompt shape).
type llmCompleter struct{ client *llm.Client }

func (l llmCompleter) Complete(ctx context.Context, system, prompt string, temperature float64, maxTokens int, responseFormat map[string]any) (string, error) {
	if l.client == nil {
		return "", fmt.Errorf("llm client is required")
	}
	res, err := l.client.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: prompt},
		},
		Temp:           temperature,
		MaxTokens:      maxTokens,
		ResponseFormat: responseFormat,
	})
	if err != nil {
		return "", err
	}
	return res.Answer, nil
}

// FileBytesReader fetches an uploaded object by name (MinIO read seam).
type FileBytesReader func(ctx context.Context, objectName string) ([]byte, error)

// SetFileBytesReader injects the file download seam used by the intake parse
// endpoints (W5-3). Without it, parse tools refuse honestly.
func (h *Agent) SetFileBytesReader(f FileBytesReader) {
	if h == nil {
		return
	}
	h.fileBytes = f
}

// intakeMaterial builds the producer source from an uploaded file: xlsx bytes
// go through the deterministic Excel reader; everything else goes through the
// docparse seam (W5-1) for text extraction.
func (h *Agent) intakeMaterial(ctx context.Context, contentType, objectName string) (aiintake.SourceMaterial, error) {
	if h == nil || h.fileBytes == nil {
		return aiintake.SourceMaterial{}, fmt.Errorf("file reader is not wired; cannot parse without a source")
	}
	data, err := h.fileBytes(ctx, objectName)
	if err != nil {
		return aiintake.SourceMaterial{}, fmt.Errorf("download file %s: %w", objectName, err)
	}
	if aiintake.IsExcelContentType(contentType) {
		text, records, locators, err := aiintake.ReadExcelContracts(data)
		if err != nil {
			return aiintake.SourceMaterial{}, fmt.Errorf("read excel: %w", err)
		}
		return aiintake.SourceMaterial{
			Text: text, ContentType: contentType, FileData: data,
			DeterministicRecords: records, EvidenceLocators: locators,
		}, nil
	}
	parser := h.DocumentParser()
	if parser == nil {
		return aiintake.SourceMaterial{}, fmt.Errorf("document parser is not wired")
	}
	doc, err := parser.Parse(ctx, docparse.Source{Data: data, Filename: objectName})
	if err != nil {
		return aiintake.SourceMaterial{}, err
	}
	return aiintake.SourceMaterial{Text: doc.Markdown, ContentType: contentType, FileData: data}, nil
}

// intakeDraft runs the in-process producer and returns the ai-intake.v1
// envelope JSON, ready for the existing Decode* consumption seam.
func (h *Agent) intakeDraft(ctx context.Context, kind, fileID, objectName, contentType, contractID string) ([]byte, error) {
	client, err := h.llm()
	if err != nil {
		return nil, err
	}
	material, err := h.intakeMaterial(ctx, contentType, objectName)
	if err != nil {
		return nil, err
	}
	cmd := aiintake.Command(kind, fileID, objectName, contentType, contractID)
	envelope, err := aiintake.Produce(ctx, "assist", kind, cmd, material, llmCompleter{client: client})
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope)
}
