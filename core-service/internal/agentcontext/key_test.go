package agentcontext

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unsafe"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// unsafePointerOf exposes an unexported field's address so AR1-G1 can mutate
// it in place (test-only helper).
func unsafePointerOf(value reflect.Value) unsafe.Pointer {
	return unsafe.Pointer(value.UnsafeAddr())
}

func principal(entityID, userID string, scope access.Scope) agenttools.Principal {
	scope.LegalEntityID = entityID
	return agenttools.Principal{UserID: userID, Scope: scope}
}

func mustKey(t *testing.T) ContextKey {
	t.Helper()
	key, err := KeyFrom(principal("entity-a", "user-1", access.Scope{}), "session-1", ClassificationProduction)
	if err != nil {
		t.Fatalf("construct key: %v", err)
	}
	return key
}

// ── 验收 1：三条不可表达 ──────────────────────────────────────────────────

func TestKeyFromRefusesIncompletePrincipal(t *testing.T) {
	cases := []struct {
		name      string
		principal agenttools.Principal
		sessionID string
		class     string
	}{
		{"empty legal entity", principal("", "user-1", access.Scope{}), "s", ClassificationProduction},
		{"global admin carries no entity", principal("", "admin-1", access.Scope{Global: true}), "s", ClassificationProduction},
		{"empty user id", principal("entity-a", "", access.Scope{}), "s", ClassificationProduction},
		{"blank user id", principal("entity-a", "   ", access.Scope{}), "s", ClassificationProduction},
		{"empty session", principal("entity-a", "user-1", access.Scope{}), "", ClassificationProduction},
		{"blank session", principal("entity-a", "user-1", access.Scope{}), "  ", ClassificationProduction},
		{"unknown classification", principal("entity-a", "user-1", access.Scope{}), "s", "staging"},
		{"empty classification", principal("entity-a", "user-1", access.Scope{}), "s", ""},
	}
	for _, tc := range cases {
		key, err := KeyFrom(tc.principal, tc.sessionID, tc.class)
		if err == nil {
			t.Fatalf("%s: key constructed %+v", tc.name, key)
		}
		if !errors.Is(err, ErrIncompleteKey) {
			t.Fatalf("%s: err=%v; want ErrIncompleteKey", tc.name, err)
		}
		if key != (ContextKey{}) {
			t.Fatalf("%s: returned a non-zero key alongside an error", tc.name)
		}
	}
}

// 参数错位即类型错误由编译器保证；这里钉住签名本身，防止将来出现
// 「三个 string」的第二个构造器。
func TestConstructorSignatureTakesPrincipal(t *testing.T) {
	constructorType := reflect.TypeOf(KeyFrom)
	if constructorType.NumIn() != 3 {
		t.Fatalf("KeyFrom has %d inputs; want 3 (principal, sessionID, classification)", constructorType.NumIn())
	}
	if got := constructorType.In(0); got != reflect.TypeOf(agenttools.Principal{}) {
		t.Fatalf("first parameter is %s; want agenttools.Principal — a string here would let callers fabricate keys", got)
	}
	for i := 1; i < 3; i++ {
		if got := constructorType.In(i); got != reflect.TypeOf("") {
			t.Fatalf("parameter %d is %s; want string", i, got)
		}
	}
}

// ── D-C12: scope 指纹三性质 ────────────────────────────────────────────────

func TestScopeFingerprintNilAndEmptyAreSameShape(t *testing.T) {
	withNil, err := KeyFrom(principal("e", "u", access.Scope{}), "s", ClassificationProduction)
	if err != nil {
		t.Fatal(err)
	}
	withEmpty, err := KeyFrom(principal("e", "u", access.Scope{
		StoreIDs: []string{}, Regions: []string{}, Brands: nil,
	}), "s", ClassificationProduction)
	if err != nil {
		t.Fatal(err)
	}
	if withNil.Cache() != withEmpty.Cache() {
		t.Fatal("nil slices and empty slices produced different cache keys")
	}
}

func TestScopeFingerprintOrderInsensitive(t *testing.T) {
	a, err := KeyFrom(principal("e", "u", access.Scope{
		StoreIDs: []string{"s1", "s2"}, Regions: []string{"east"},
	}), "s", ClassificationProduction)
	if err != nil {
		t.Fatal(err)
	}
	b, err := KeyFrom(principal("e", "u", access.Scope{
		StoreIDs: []string{"s2", "s1"}, Regions: []string{"east"},
	}), "s", ClassificationProduction)
	if err != nil {
		t.Fatal(err)
	}
	if a.Cache() != b.Cache() {
		t.Fatal("same scope with different slice ordering produced different keys")
	}
}

func TestScopeFingerprintChangesOnAnyDimension(t *testing.T) {
	base, err := KeyFrom(principal("e", "u", access.Scope{
		StoreIDs:        []string{"s1"},
		Regions:         []string{"east"},
		Brands:          []string{"b"},
		Plants:          []string{"p"},
		ProductionLines: []string{"l"},
		EquipmentIDs:    []string{"q"},
	}), "s", ClassificationProduction)
	if err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name   string
		entity string // principal() re-stamps scope.LegalEntityID from this
		mutate func(*access.Scope)
	}{
		{"store added", "e", func(s *access.Scope) { s.StoreIDs = append(s.StoreIDs, "s2") }},
		{"store removed", "e", func(s *access.Scope) { s.StoreIDs = nil }},
		{"region changed", "e", func(s *access.Scope) { s.Regions = []string{"west"} }},
		{"brand changed", "e", func(s *access.Scope) { s.Brands = []string{"other"} }},
		{"plant changed", "e", func(s *access.Scope) { s.Plants = []string{"p2"} }},
		{"line changed", "e", func(s *access.Scope) { s.ProductionLines = []string{"l2"} }},
		{"equipment changed", "e", func(s *access.Scope) { s.EquipmentIDs = []string{"q2"} }},
		{"legal entity moved", "entity-b", func(s *access.Scope) {}},
	}
	for _, mutation := range mutations {
		scope := access.Scope{
			StoreIDs:        []string{"s1"},
			Regions:         []string{"east"},
			Brands:          []string{"b"},
			Plants:          []string{"p"},
			ProductionLines: []string{"l"},
			EquipmentIDs:    []string{"q"},
		}
		mutation.mutate(&scope)
		changed, err := KeyFrom(principal(mutation.entity, "u", scope), "s", ClassificationProduction)
		if err != nil {
			t.Fatalf("%s: construct: %v", mutation.name, err)
		}
		if changed.Cache() == base.Cache() {
			t.Fatalf("%s: cache key did not change after scope mutation", mutation.name)
		}
	}
}

// ── D-C20: 分类参与键（底线 2 的上下文落点）────────────────────────────────

func TestClassificationChangesCache(t *testing.T) {
	p := principal("e", "u", access.Scope{})
	a, err := KeyFrom(p, "s", ClassificationProduction)
	if err != nil {
		t.Fatal(err)
	}
	b, err := KeyFrom(p, "s", ClassificationSimulated)
	if err != nil {
		t.Fatal(err)
	}
	c, err := KeyFrom(p, "s", ClassificationMixed)
	if err != nil {
		t.Fatal(err)
	}
	if a.Cache() == b.Cache() || a.Cache() == c.Cache() || b.Cache() == c.Cache() {
		t.Fatal("different classifications collapsed to one cache key")
	}
	again, err := KeyFrom(p, "s", ClassificationMixed)
	if err != nil || again.Cache() != c.Cache() {
		t.Fatalf("construction not deterministic: %q vs %q", again.Cache(), c.Cache())
	}
}

// ── D-C11: 无 String() 方法 ────────────────────────────────────────────────

func TestContextKeyHasNoStringMethod(t *testing.T) {
	var key any = ContextKey{}
	if _, ok := key.(fmt.Stringer); ok {
		t.Fatalf("ContextKey implements fmt.Stringer — implicit %%v would leak tenant/user ids into logs (D-C11)")
	}
	stringerType := reflect.TypeOf((*interface{ String() string })(nil)).Elem()
	if reflect.TypeOf(ContextKey{}).Implements(stringerType) {
		t.Fatal("ContextKey has a String() method")
	}
}

// ── AR1-G1: 全字段参与缓存键（反射遍历）────────────────────────────────────

// TestEveryFieldParticipatesInCache mutates every field of a fully-built key
// and requires Cache() to change. Written for the next person who adds a
// dimension: forget it in Cache() and this goes red.
func TestEveryFieldParticipatesInCache(t *testing.T) {
	base := mustKey(t)
	typ := reflect.TypeOf(base)
	if typ.NumField() < 5 {
		t.Fatalf("ContextKey unexpectedly shrank to %d fields; the isolation contract lost dimensions", typ.NumField())
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		mutated := base
		fieldValue := reflect.ValueOf(&mutated).Elem().Field(i)
		switch fieldValue.Kind() {
		case reflect.String:
			if fieldValue.String() == "" {
				t.Fatalf("field %s is empty on a fully-constructed key; AR1-G1 cannot probe it", field.Name)
			}
			// Unexported fields are not addressable through plain reflection;
			// NewAt re-arms the setter for the same memory.
			settable := reflect.NewAt(fieldValue.Type(), unsafePointerOf(fieldValue)).Elem()
			settable.SetString(settable.String() + "-mutated")
		default:
			t.Fatalf("field %s has unhandled kind %s — extend this guard's mutation table when adding it", field.Name, fieldValue.Kind())
		}
		if mutated.Cache() == base.Cache() {
			t.Errorf("field %s does not participate in Cache(): two different contexts share one key", field.Name)
		}
	}
}

// ── AR1-G2: 消费方不得旁路（可扩展骨架）────────────────────────────────────

// contextConsumers lists directories whose exported functions may only take an
// agentcontext.ContextKey — never bare identifier strings. Paths are relative
// to this package's directory (go test runs with the package dir as CWD).
// AR2 Session Manager is registered; AR3 Context Assembler / AR6 Memory join
// as they land.
var contextConsumers = map[string][]string{
	"../sessionmanager":   nil,
	"../contextassembler": nil,
}

var bareParamPattern = regexp.MustCompile(
	`func [A-Z]\w*\([^)]*\b(entityID|userID|sessionID|legalEntityID)\s+string[^)]*\)`)

func TestContextConsumersTakeNoBareIdentifiers(t *testing.T) {
	for dir := range contextConsumers {
		violations, err := scanForBareIdentifierParams(dir)
		if err != nil {
			t.Fatalf("scan %s: %v", dir, err)
		}
		if len(violations) > 0 {
			t.Fatalf("consumer packages must take agentcontext.ContextKey, never bare identifier strings:\n  %s",
				strings.Join(violations, "\n  "))
		}
	}
}

// Reverse fixture for the skeleton: the scanner must catch a planted exported
// function with a bare userID parameter. Runs against a temp dir so the still-
// empty consumer list cannot mask scanner rot.
func TestConsumerScannerDetectsBareIdentifierParams(t *testing.T) {
	dir := t.TempDir()
	fixture := "package fake\n\nfunc GetHistory(userID string) []string { return nil }\n"
	if err := os.WriteFile(filepath.Join(dir, "fake.go"), []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	violations, err := scanForBareIdentifierParams(dir)
	if err != nil || len(violations) == 0 {
		t.Fatalf("scanner failed to detect planted bare param: violations=%v err=%v", violations, err)
	}
	if !strings.Contains(violations[0], "userID") {
		t.Fatalf("violation does not name the offending parameter: %v", violations)
	}
}

// Reverse fixture against every REGISTERED consumer package: plant a bare-
// identifier function beside its real sources, require red, remove it so CI
// sees only the clean tree (same pattern as the vendor import guard). This
// proves the scanner actually scans each registration — a typo'd path would
// otherwise scan nothing and stay silently green.
func TestConsumerScannerDetectsBareParamsInRegisteredPackages(t *testing.T) {
	if len(contextConsumers) == 0 {
		t.Fatal("no consumers registered — the AR1-G2 guard is scanning nothing")
	}
	for dir := range contextConsumers {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("registered consumer %q does not exist: %v", dir, err)
		}
		fixture := filepath.Join(dir, "zz_ar1g2_fixture_test_bare.go")
		content := "package sessionmanager\n\nfunc GetSession(sessionID string) string { return sessionID }\n"
		if err := os.WriteFile(fixture, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		violations, err := scanForBareIdentifierParams(dir)
		os.Remove(fixture)
		if err != nil {
			t.Fatalf("scan %s: %v", dir, err)
		}
		if len(violations) == 0 {
			t.Fatalf("scanner failed to flag the planted bare param in registered consumer %s", dir)
		}
		found := false
		for _, violation := range violations {
			if strings.Contains(violation, "sessionID") {
				found = true
			}
		}
		if !found {
			t.Fatalf("violation for %s does not name sessionID: %v", dir, violations)
		}
	}
}

// scanForBareIdentifierParams reads every non-test Go file under dir and
// reports lines whose exported-function signatures carry bare
// identifier-string parameters.
func scanForBareIdentifierParams(dir string) ([]string, error) {
	violations := []string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for index, line := range strings.Split(string(content), "\n") {
			if bareParamPattern.MatchString(line) {
				violations = append(violations,
					fmt.Sprintf("%s:%d: %s", path, index+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	return violations, err
}
