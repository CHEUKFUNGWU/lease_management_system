package handlers

import (
	"testing"

	"github.com/lease-management-system/core-service/internal/repository"
)

func TestReverseMessagesDoesNotMutateRepositoryOrder(t *testing.T) {
	original := []*repository.AIChatMessage{{ID: "m1"}, {ID: "m2"}, {ID: "m3"}}

	reversed := reverseMessages(original)

	if got, want := reversed[0].ID, "m3"; got != want {
		t.Fatalf("reversed[0] = %s, want %s", got, want)
	}
	if got, want := original[0].ID, "m1"; got != want {
		t.Fatalf("repository order mutated: original[0] = %s, want %s", got, want)
	}
}
