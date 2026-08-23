package aiagent

// A-2 注册完整性守卫（Batch A）：
//
// 背景：fpna.assumptions.suggest 曾因 InputSchema 多一个右花括号被
// Registry.Register 拒绝（"input_schema must be a JSON object"），而 agent.go
// 的每个注册点都写成 `if err := registry.Register(d); err == nil`，错误被静默
// 丢弃——工具在 AGENTS.md 有记录、有自己的单元测试、代码看着正常，但从未进入
// registry。commit 62db083 修了字面量本身，但注册过程仍然可以静默丢错。
//
// 本文件两条独立路径守卫注册完整性：
//   路径 1（collector 账本）：agent.registrationFailed == 0
//   路径 2（真实运行时枚举）：Runtime.Describe 走 registry map + policy +
//     discoverability，与 collector 的 attempted 数对照。
// 若注册失败被静默吞掉（fail-fast 被删、账本被剥离），两条路径各自独立变红；
// 不与 collector 自比（风险红线 12：恒真的勾稽不算检查）。

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/closereadiness"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
	"github.com/lease-management-system/core-service/internal/services/reporting"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

// --- 生产端口桩：镜像 cmd/api/main.go 的全接线，只是仓库换成零值指针、其余接口换成桩。

type registrationPerfStub struct{}

func (registrationPerfStub) Overview(context.Context, access.EntityFilter, string) (*repository.PerformanceOverview, error) {
	return nil, nil
}
func (registrationPerfStub) ListStores(context.Context, access.EntityFilter, string, string) ([]*repository.StoreOperatingFact, error) {
	return nil, nil
}
func (registrationPerfStub) ListEquipmentFacts(context.Context, access.EntityFilter, string, string, string) ([]*repository.EquipmentOperatingFact, error) {
	return nil, nil
}
func (registrationPerfStub) ListActions(context.Context, access.EntityFilter, string, string, string) ([]*repository.FPnAActionItem, error) {
	return nil, nil
}
// OperatingFactsRepository 同时实现写口（main.go 把 operatingFactsRepo 传给
// performance 参数），桩必须同样实现，否则全表面少 4 个 draft 工具，守卫量级不足。
func (registrationPerfStub) CreateAction(context.Context, *repository.FPnAActionItem) (*repository.FPnAActionItem, error) {
	return nil, nil
}
func (registrationPerfStub) CreateScenarioDraft(context.Context, *repository.FPnAScenarioDraft) (*repository.FPnAScenarioDraft, error) {
	return nil, nil
}

type registrationGovStub struct{}

func (registrationGovStub) CreateMemo(context.Context, *repository.FPnADecisionMemo) (*repository.FPnADecisionMemo, error) {
	return nil, nil
}

type registrationCloseStub struct{}

func (registrationCloseStub) Evaluate(context.Context, closereadiness.Command) (*closereadiness.Result, error) {
	return &closereadiness.Result{}, nil
}

// registrationRetailStub 同时满足 RetailOperationsReader 与 finadapter.FactsSource
//（两者 QueryFacts 签名相同；生产里 retailKPIRepo 同时扮演这两个端口）。
type registrationRetailStub struct{}

func (registrationRetailStub) QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error) {
	return nil, nil
}

type registrationSensStub struct{}

func (registrationSensStub) Sensitivity(context.Context, string, *float64, []float64) (reporting.ProjectionResult, error) {
	return reporting.ProjectionResult{}, nil
}

type registrationFillStub struct{}

func (registrationFillStub) ReadObject(context.Context, string) ([]byte, error) { return nil, nil }

type registrationBudgetStub struct{}

func (registrationBudgetStub) ReadVariance(context.Context, access.EntityFilter, string, string) (any, error) {
	return nil, nil
}

type registrationCashflowStub struct{}

func (registrationCashflowStub) ReadScenario(context.Context, string, agenttooldefs.CashflowScenarioArguments) (any, error) {
	return nil, nil
}

type registrationRenewalStub struct{}

func (registrationRenewalStub) ReadDecisions(context.Context, access.EntityFilter, string) (any, error) {
	return nil, nil
}

// productionWire mirrors the cmd/api/main.go production wiring with every port
// non-nil. Any silent registration drop in that surface must red the guard.
func productionWire() *Agent {
	return newAgent(
		repository.NewContractRepository(nil),
		repository.NewMonthlyClosingRepository(nil),
		repository.NewEventRepository(nil),
		registrationPerfStub{},
		registrationCloseStub{},
		&agenttooldefs.ControlReaders{
			Budget:   registrationBudgetStub{},
			Cashflow: registrationCashflowStub{},
			Renewal:  registrationRenewalStub{},
		},
		registrationGovStub{},
		registrationRetailStub{},
		registrationSensStub{},
		registrationFillStub{},
		registrationStorePnlStub{},
		repository.NewFinModelRepository(nil),
		registrationRetailStub{}, // facts
		repository.NewFPnAGovernanceRepository(nil),
		registrationFactsStub{},
		registrationReportStub{},
		draftapp.NewService(nil, nil),
	)
}

// registrationReportStub satisfies agenttooldefs.ReportReader so the full
// production surface exercises the report tool registrations.
type registrationReportStub struct{}

func (registrationReportStub) Project(context.Context, reporting.Mode, reporting.ProjectionRequest) (reporting.ProjectionResult, error) {
	return reporting.ProjectionResult{}, nil
}

func (registrationReportStub) ClosePack(context.Context, reporting.Mode, string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (registrationReportStub) ListPlanVersions(context.Context, access.EntityFilter, string, string, string) ([]*repository.FPnAPlanVersion, error) {
	return nil, nil
}

func (registrationReportStub) ListAssumptions(context.Context, access.EntityFilter, string) ([]*repository.FPnAAssumptionVersion, error) {
	return nil, nil
}

func (registrationReportStub) ListScenarioDrafts(context.Context, access.EntityFilter, int) ([]*repository.FPnAScenarioDraft, error) {
	return nil, nil
}

// registrationFactsStub satisfies agenttooldefs.OperatingFactsReader so the
// full production surface exercises the operating-facts tool registrations.
type registrationFactsStub struct{}

func (registrationFactsStub) ListStores(context.Context, access.EntityFilter, string, string) ([]*repository.StoreOperatingFact, error) {
	return nil, nil
}

func (registrationFactsStub) ListRetailStoreDayFactsPage(context.Context, access.EntityFilter, string, string, []string, string, int, int) (*repository.RetailStoreDayFactsPage, error) {
	return &repository.RetailStoreDayFactsPage{}, nil
}

// registrationStorePnlStub satisfies agenttooldefs.StorePnlReader so the full
// production surface exercises the fpna.store_pnl.read registration.
type registrationStorePnlStub struct{}

func (registrationStorePnlStub) Project(context.Context, agenttooldefs.StorePnlQuery) (*storepnl.StorePnl, error) {
	return &storepnl.StorePnl{}, nil
}

func TestAgentToolRegistrationCompleteness(t *testing.T) {
	agent := productionWire()

	// 路径 1：collector 账本无失败。成功数 == 尝试数。
	if agent.registrationFailed != 0 {
		t.Fatalf("registration accounting shows %d failed attempt(s) out of %d: registered tools must equal attempted tools, no drop may be silent",
			agent.registrationFailed, agent.registrationAttempted)
	}
	if agent.registrationAttempted == 0 {
		t.Fatal("registration accounting is empty — the wiring must attempt tools")
	}

	// 路径 2：真实运行时的 Describe 枚举——走 registry map、policy、权限可见性
	// ——与 collector 的 attempted 对照。这是独立于 collector 的来源：注册了但
	// 没被计数、或计数了但没落进 registry 的工具，都只会在这条路径上现形。
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "x", Role: "admin", Permissions: []string{"*:*"}},
		RunID:     "x",
	})
	descriptors, err := agent.ToolRuntime().Describe(ctx, agenttools.ToolFilter{})
	if err != nil {
		t.Fatalf("Runtime.Describe: %v", err)
	}
	if len(descriptors) != agent.registrationAttempted {
		t.Fatalf("registry holds %d tools (Runtime.Describe) but registration accounting attempted %d: a tool was silently dropped or double-counted",
			len(descriptors), agent.registrationAttempted)
	}

	// 统计口径（AGENTS.md）：报告工具数一律运行时枚举，不 grep、不引用文档数字。
	reads, drafts := 0, 0
	for _, d := range descriptors {
		if d.Level == agenttools.LevelRead {
			reads++
		} else {
			drafts++
		}
	}
	t.Logf("registered tools: %d total (%d read / %d write)", len(descriptors), reads, drafts)
}

// A-1 机制单元测试：fail() 的 panic 材料必须带上工具名与底层 Register 错误原文。
// 复现 62db083 的成因（InputSchema 多一个右花括号），验证 collector 把
// name@version 和 "input_schema must be a JSON object" 原文一起保留。
func TestRegisterCollectorFailFastCarriesToolNameAndOriginalError(t *testing.T) {
	registry := agenttools.NewRegistry()
	collector := &registerCollector{registry: registry}

	// 基线：一个合法定义必须先注册成功。
	collector.add(agenttooldefs.NewS1GenerateDefinition())
	if collector.attempted != 1 || collector.succeeded != 1 {
		t.Fatalf("baseline registration must succeed: attempted=%d succeeded=%d", collector.attempted, collector.succeeded)
	}
	if err := collector.fail(); err != nil {
		t.Fatalf("baseline collector must not fail: %v", err)
	}

	// 坏定义：InputSchema 字面量多一个右花括号（62db083 的原始成因）。
	broken := agenttooldefs.NewS1GenerateDefinition()
	broken.Descriptor.Name = "test.broken.schema"
	broken.Descriptor.InputSchema = json.RawMessage(`{"type":"object"}}`)
	collector.add(broken)

	if collector.attempted != 2 || collector.succeeded != 1 {
		t.Fatalf("broken registration must be attempted but not succeed: attempted=%d succeeded=%d", collector.attempted, collector.succeeded)
	}
	err := collector.fail()
	if err == nil {
		t.Fatal("collector must fail when a registration does not land")
	}
	message := err.Error()
	if !strings.Contains(message, "test.broken.schema@v1") {
		t.Fatalf("fail-fast message must carry the tool name: %q", message)
	}
	if !strings.Contains(message, "input_schema must be a JSON object") {
		t.Fatalf("fail-fast message must carry the original Register error verbatim: %q", message)
	}
}
