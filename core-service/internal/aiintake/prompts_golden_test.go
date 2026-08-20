package aiintake

// W5-3 门 B prompt-fidelity gate: the four prompt templates must render
// byte-for-byte identical to the golden files recorded from the OLD Python
// producer (see ai-service/scripts/record_corr2_baseline.py). A wording, field
// order or constraint-sentence change makes this red — the golden file is the
// contract, and changing it must be justified in the PR, never silent.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPromptGoldenFidelity(t *testing.T) {
	// The exact input the recorder used to render the golden files.
	input := "租赁合同 编号 GOLDEN-001\n承租方：示例公司\n出租方：示例置地\n租赁起始日：2026-01-01\n"
	cases := []struct {
		name string
		want string
	}{
		{"prompt-contract", contractPrompt(input)},
		{"prompt-payment_schedule", paymentPrompt(input)},
		{"prompt-event", eventPrompt(input, "c-001")},
		{"prompt-contract_batch", contractBatchPrompt(input)},
	}
	dir := filepath.Join("..", "agentseval", "testdata", "corr2", "golden")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			golden, err := os.ReadFile(filepath.Join(dir, c.name+".golden.txt"))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if string(golden) != c.want {
				t.Fatalf("Go prompt diverges from the recorded Python golden (%d vs %d bytes): any wording change must update the golden file and be justified in the PR",
					len(c.want), len(golden))
			}
		})
	}
}

// The prompt renderers are pure and deterministic: same input, same bytes.
func TestPromptDeterministic(t *testing.T) {
	input := "X"
	if contractPrompt(input) != contractPrompt(input) {
		t.Fatal("contract prompt must be deterministic")
	}
	if eventPrompt(input, "c-1") != eventPrompt(input, "c-1") {
		t.Fatal("event prompt must be deterministic")
	}
	if eventPrompt(input, "c-1") == eventPrompt(input, "c-2") {
		t.Fatal("event prompt must reflect the contract id")
	}
}
