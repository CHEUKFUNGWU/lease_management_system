package access

import "testing"

func TestScopeDoesNotExpandBeyondAssignedLegalEntity(t *testing.T) {
	scope := Scope{LegalEntityID: "le-001"}

	if !scope.AllowsContract(ContractAttributes{LegalEntityID: "le-001", StoreID: "store-1"}) {
		t.Fatal("expected full access inside assigned legal entity when no narrower scope exists")
	}
	if scope.AllowsContract(ContractAttributes{LegalEntityID: "le-002", StoreID: "store-1"}) {
		t.Fatal("expected access outside assigned legal entity to be denied")
	}
}

func TestAssignedDataScopesNarrowLegalEntityAccess(t *testing.T) {
	scope := Scope{
		LegalEntityID: "le-001",
		StoreIDs:      []string{"store-1"},
		Regions:       []string{"east"},
		Brands:        []string{"brand-a"},
	}

	allowed := ContractAttributes{LegalEntityID: "le-001", StoreID: "store-1", Region: "east", Brand: "brand-a"}
	if !scope.AllowsContract(allowed) {
		t.Fatal("expected contract matching every assigned dimension to be allowed")
	}

	denied := []ContractAttributes{
		{LegalEntityID: "le-001", StoreID: "store-2", Region: "east", Brand: "brand-a"},
		{LegalEntityID: "le-001", StoreID: "store-1", Region: "west", Brand: "brand-a"},
		{LegalEntityID: "le-001", StoreID: "store-1", Region: "east", Brand: "brand-b"},
	}
	for _, contract := range denied {
		if scope.AllowsContract(contract) {
			t.Fatalf("expected contract outside a narrowing dimension to be denied: %#v", contract)
		}
	}
}
