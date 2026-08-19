package persist_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lease-management-system/core-service/internal/finmodel"
	"github.com/lease-management-system/core-service/internal/finmodel/persist"
)

func TestWriterRefusesFailedTieOuts(t *testing.T) {
	writer := persist.NewRunWriter(nil)
	result := &finmodel.RunResult{TieOutStatus: "failed"}
	err := writer.Persist(context.Background(), finmodel.ModelDef{}, result, "model-1", "k1", nil)
	if !errors.Is(err, persist.ErrTieOutFailed) {
		t.Fatalf("failed tie-outs must block Persist, got %v", err)
	}
}
