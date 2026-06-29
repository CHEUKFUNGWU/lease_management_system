package repository

import "testing"

func TestCompletedWithErrorsIsTerminalBatchStatus(t *testing.T) {
	if !isTerminalBatchStatus("completed_with_errors") {
		t.Fatal("completed_with_errors must set completed_at")
	}
}
