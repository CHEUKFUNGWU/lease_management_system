package handlers

import "testing"

func TestAIReviewActionPermission(t *testing.T) {
	tests := []struct {
		action    string
		required  string
		supported bool
	}{
		{action: "confirm", required: "confirm", supported: true},
		{action: "import", required: "confirm", supported: true},
		{action: "create_draft", required: "confirm", supported: true},
		{action: "reject", required: "review", supported: true},
		{action: "skip", required: "review", supported: true},
		{action: "unknown", supported: false},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			required, supported := aiReviewActionPermission(tt.action)
			if required != tt.required || supported != tt.supported {
				t.Fatalf("expected (%q, %t), got (%q, %t)", tt.required, tt.supported, required, supported)
			}
		})
	}
}
