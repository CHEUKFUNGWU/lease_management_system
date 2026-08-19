package template

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryStoreFreezeSemantics(t *testing.T) {
	store := NewMemoryStore()
	def := TemplateDef{Name: "frozen", Version: 1, Rows: []RowDef{
		{Key: "rev", Label: "收入", Kind: RowLink, Basis: BasisShared, Source: "fact.revenue"},
	}}
	if _, err := store.Save(context.Background(), def); err != nil {
		t.Fatal(err)
	}
	// Same version again → frozen.
	if _, err := store.Save(context.Background(), def); !errors.Is(err, ErrVersionFrozen) {
		t.Fatalf("second save of the same version must be frozen, got %v", err)
	}
	// New version = the edit path.
	def.Version = 2
	if _, err := store.Save(context.Background(), def); err != nil {
		t.Fatalf("new version must save, got %v", err)
	}
	got, err := store.Get(context.Background(), "frozen", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Major != 1 {
		t.Fatalf("version 1 must read back intact, got %d", got.Major)
	}
	list, err := store.List(context.Background())
	if err != nil || len(list) != 2 {
		t.Fatalf("List must return both versions, got %v err=%v", list, err)
	}
}
