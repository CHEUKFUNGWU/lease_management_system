package aiagent

// C1（架构重构任务书 2026-08-26）：工具注册面按三命名空间拆成域 bundle，
// agent.go 只组装。本文件同时承载构造参数的归组结构（AgentPorts）——仿
// agenttools.ControlReaders 的分组先例：一个字段一个能力角色，生产仓库在
// main.go 只出现一次（旧位置参数构造函数把 operatingFactsRepo /
// retailKPIRepo / fpnaGovernanceRepo 各传两遍）。
//
// 红线：Descriptor 内容（名、权限、schema）与 fail-fast 语义原样保留；
// 工具名一个都不改；未装 guard 回落 Evaluate 的双路径语义不动。

import (
	"github.com/lease-management-system/core-service/internal/agenttools"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
	finadapter "github.com/lease-management-system/core-service/internal/finmodel/adapter"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
)

// OperatingFactsSource is what one operating-facts store provides the tool
// surface: the performance read family and the B-3 fact-read family. One
// repository, two interface views — the constructor takes it once.
type OperatingFactsSource interface {
	agenttooldefs.PerformanceReader
	agenttooldefs.OperatingFactsReader
}

// RetailKPISource is what one retail KPI store provides: the retail
// operations face and the fin-model facts face.
type RetailKPISource interface {
	agenttooldefs.RetailOperationsReader
	finadapter.FactsSource
}

// AgentPorts groups every external capability the tool surface registers
// against. Field order follows the three namespaces; shared stores sit under
// 「共享」 with a comment naming every consumer bundle.
type AgentPorts struct {
	// —— lease.* ——合同、月结、事件、草稿层、文件摄取、关账就绪、报表、敏感性。
	Contracts       *repository.ContractRepository
	Closing         *repository.MonthlyClosingRepository
	Events          *repository.EventRepository
	DraftServices   []*draftapp.Service
	FileIngest      agenttooldefs.IngestFileReader
	CloseReadiness  agenttooldefs.CloseReadinessReader
	Reports         agenttooldefs.ReportReader
	RateSensitivity agenttooldefs.SensitivityReader

	// —— fpna.* ——决策备忘、计划版本（capex）、三表模型、单店利润表。
	Governance   agenttooldefs.DecisionMemoDraftWriter
	Plans        *repository.FPnAGovernanceRepository
	FinModelRepo *repository.FinModelRepository
	StorePnl     agenttooldefs.StorePnlReader

	// —— 共享（被多个命名空间消费，各出现一次）——
	// Controls: budget/cashflow 归 fpna，renewal.decisions 归 lease。
	Controls *agenttooldefs.ControlReaders
	// OperatingFacts: performance 家族（lease.portfolio.summary、fpna 的
	// pre-read/actions 家族、retail 的门店/设备业绩读）+ B-3 经营事实只读面。
	OperatingFacts OperatingFactsSource
	// RetailKPI: 零售运营读面 + 三表模型事实源。
	RetailKPI RetailKPISource
	// Ecommerce: 电商独立站模式（ecommerce-dtc-mode-v1）读面——
	// retail.site_* 三个 + fpna.site_pnl/settlement 两读 + 两个草稿写口。
	Ecommerce agenttooldefs.EcomReader
}

// registerLeaseTools registers the lease.* surface: contract/closing/event
// reads, the AI intake draft pipeline, working papers, monthly closing reads,
// the pre-signature simulation family and the report/sensitivity reads.
func registerLeaseTools(h *Agent, collector *registerCollector, p AgentPorts) {
	if p.Contracts != nil {
		collector.add(agenttooldefs.NewContractSearchDefinition(p.Contracts))
		collector.add(agenttooldefs.NewContractGetDefinition(p.Contracts))
		if p.Closing != nil {
			collector.add(agenttooldefs.NewMeasurementListDefinition(p.Contracts, p.Closing))
			collector.add(agenttooldefs.NewJournalListDefinition(p.Contracts, p.Closing))
		}
		if p.Events != nil {
			collector.add(agenttooldefs.NewEventListDefinition(p.Contracts, p.Events))
		}
	}
	for _, definition := range h.fileParseDefinitions() {
		collector.add(definition)
	}
	collector.add(agenttooldefs.NewDocTriageDefinition(nil))
	collector.add(agenttooldefs.NewS1GenerateDefinition())
	// The fill seam registers without a file reader for now (D-D2): the tool
	// refuses honestly until W5 wires minio-go into core-service.
	collector.add(agenttooldefs.NewRetailIngestPreviewDefinition(p.FileIngest))
	// agent-universal-pagefill-v1 P0-B①：GL 试算平衡表预填（同一文件读取接缝）。
	collector.add(agenttooldefs.NewTrialBalanceFillDefinition(p.FileIngest))
	// agent-universal-pagefill-v1 P0-B①：付款计划预填（租金表 → 合同工作台表单）。
	collector.add(agenttooldefs.NewPaymentScheduleFillDefinition(p.FileIngest))
	// B-2：月结只读面（跑批状态 / 期间级分录预览 / 有分录期间 / 锁账状态），
	// 仅依赖 mcRepo。写口（生成、审批、过账、红冲、锁账、解锁、ERP 回写）
	// 一律不开放给 Agent——那是审批与锁账控制的核心。
	if p.Closing != nil {
		collector.add(agenttooldefs.NewMonthlyClosingBatchesDefinition(p.Closing))
		collector.add(agenttooldefs.NewMonthlyClosingEntriesPreviewDefinition(p.Closing))
		collector.add(agenttooldefs.NewMonthlyClosingPeriodsDefinition(p.Closing))
		collector.add(agenttooldefs.NewMonthlyClosingLockStatusDefinition(p.Closing))
	}
	if len(p.DraftServices) > 0 && p.DraftServices[0] != nil {
		collector.add(agenttooldefs.NewContractDraftDefinition(p.DraftServices[0]))
		collector.add(agenttooldefs.NewPaymentScheduleDraftDefinition(p.DraftServices[0]))
		if p.Events != nil {
			collector.add(agenttooldefs.NewEventDraftDefinition(p.DraftServices[0]))
		}
	}
	// lease.portfolio.summary 挂在经营事实端口上，但工具名是 lease 命名空间。
	if p.OperatingFacts != nil {
		collector.add(agenttooldefs.NewPortfolioSummaryDefinition(p.OperatingFacts))
	}
	for _, definition := range []agenttools.ToolDefinition{
		agenttooldefs.NewDealSimulationDefinition(),
		agenttooldefs.NewPreDealSimulationDefinition(),
		agenttooldefs.NewRenewalSimulationDefinition(),
	} {
		collector.add(definition)
	}
	if p.CloseReadiness != nil {
		collector.add(agenttooldefs.NewCloseReadinessDefinition(p.CloseReadiness))
	}
	if p.Controls != nil {
		collector.add(agenttooldefs.NewRenewalDecisionDefinition(p.Controls.Renewal))
	}
	// B-4：报表只读面（摊销/负债滚动/现金流预测、披露与关账包、合同汇总
	// 与准则对比、单价对比、标签）。导出路由（reports:export）不包——文件类
	// Artifact 走 LevelDraft + Review Gate，与只读报表不是一件事。
	if p.Reports != nil {
		collector.add(agenttooldefs.NewReportScheduleDefinition(p.Reports))
		collector.add(agenttooldefs.NewReportDisclosurePackageDefinition(p.Reports))
		collector.add(agenttooldefs.NewReportContractViewDefinition(p.Reports))
		collector.add(agenttooldefs.NewReportUnitPriceDefinition(p.Reports))
		collector.add(agenttooldefs.NewReportTagsDefinition(p.Reports))
	}
	if p.RateSensitivity != nil {
		collector.add(agenttooldefs.NewSensitivityDefinition(p.RateSensitivity))
	}
}

// registerFPnATools registers the fpna.* surface: the three-statement model,
// assumption suggestions (draft-only), decision memos, the FP&A action
// family, the control-plane readers and the store P&L projection.
func registerFPnATools(collector *registerCollector, p AgentPorts) {
	// SM7：三表模型工具注册。生产接线注册真实端口；无仓库（测试/轻量适配器）
	// 时才注册 nil 版（工具诚实拒绝，绝不让 nil 注册挡住生产端口——P0-8）。
	if p.FinModelRepo == nil {
		collector.add(agenttooldefs.NewStatementModelReadDefinition(nil))
		collector.add(agenttooldefs.NewStatementModelEvaluateDefinition(nil))
		collector.add(agenttooldefs.NewFinModelPaperDefinition(nil))
		// 假设建议等写口（S4）：未接线保持诚实拒绝。写入路径全部 draft-only。
		collector.add(agenttooldefs.NewAssumptionSuggestionDefinition(nil))
		collector.add(agenttooldefs.NewAssumptionSuggestionBatchDefinition(nil))
		collector.add(agenttooldefs.NewModelDiffMemoDefinition(nil))
		// F1：科目树草稿生成。未接线保持诚实拒绝。
		collector.add(agenttooldefs.NewCoaSuggestTemplateDefinition(nil))
	} else {
		writer := finadapter.NewDraftWriter(p.FinModelRepo)
		var plansCapex finadapter.CapexSource
		if p.Plans != nil {
			plansCapex = p.Plans
		}
		var modelFacts finadapter.FactsSource
		if p.RetailKPI != nil {
			modelFacts = p.RetailKPI
		}
		ports := finadapter.NewPortsBuilder(p.FinModelRepo, modelFacts).WithSources(p.Closing, nil, plansCapex)
		reader := finadapter.NewStatementReader(p.FinModelRepo)
		collector.add(agenttooldefs.NewStatementModelReadDefinition(reader))
		collector.add(agenttooldefs.NewStatementModelEvaluateDefinition(ports))
		collector.add(agenttooldefs.NewFinModelPaperDefinition(ports))
		collector.add(agenttooldefs.NewAssumptionSuggestionDefinition(writer))
		collector.add(agenttooldefs.NewAssumptionSuggestionBatchDefinition(writer))
		// F1：科目树草稿生成，draft-only，source=ai_suggestion。
		collector.add(agenttooldefs.NewCoaSuggestTemplateDefinition(agenttooldefs.NewCoaTemplateStore(p.FinModelRepo)))
		if p.Plans != nil {
			collector.add(agenttooldefs.NewModelDiffMemoDefinition(p.Plans))
		} else {
			collector.add(agenttooldefs.NewModelDiffMemoDefinition(nil))
		}
	}
	if p.OperatingFacts != nil {
		collector.add(agenttooldefs.NewManagementPreReadDefinition(p.OperatingFacts))
		collector.add(agenttooldefs.NewActionListDefinition(p.OperatingFacts))
		if writer, ok := p.OperatingFacts.(agenttooldefs.ActionDraftWriter); ok {
			collector.add(agenttooldefs.NewActionDraftDefinition(writer))
			collector.add(agenttooldefs.NewExplanationDraftDefinition(writer))
			collector.add(agenttooldefs.NewMeetingActionDraftDefinition(writer))
		}
		if writer, ok := p.OperatingFacts.(agenttooldefs.ScenarioDraftWriter); ok {
			collector.add(agenttooldefs.NewScenarioDraftDefinition(writer))
		}
	}
	if p.Governance != nil {
		collector.add(agenttooldefs.NewDecisionMemoDraftDefinition(p.Governance))
	}
	collector.add(agenttooldefs.NewDecisionSummaryDefinition())
	if p.Controls != nil {
		collector.add(agenttooldefs.NewBudgetVarianceDefinition(p.Controls.Budget))
		collector.add(agenttooldefs.NewCashflowScenarioDefinition(p.Controls.Cashflow))
	}
	// B-1：单店利润表投影。P0-8：storePnl 未接线时不注册（诚实缺席，
	// 绝不注册 nil 端口版挡住真实端口）；接线后只有读工具，零写入。
	if p.StorePnl != nil {
		collector.add(agenttooldefs.NewStorePnlReadDefinition(p.StorePnl))
	}
	// 电商独立站模式（spec §5 七个工具一次定全）：fpna.site_pnl.read、
	// fpna.settlement.read 两个读 + fpna.settlement_recon_draft.create 与
	// fpna.ecom_assumption.suggest 两个草稿写口。写口恒 draft-only
	// （source=ai_suggestion / memo draft）；approved-only 读不回采。
	if p.Ecommerce != nil {
		collector.add(agenttooldefs.NewSitePnlDefinition(p.Ecommerce))
		collector.add(agenttooldefs.NewSettlementReadDefinition(p.Ecommerce))
	}
	collector.add(agenttooldefs.NewSettlementReconDraftDefinition(p.Governance))
	writer2 := agenttooldefs.AssumptionDraftWriter(nil)
	if p.FinModelRepo != nil {
		writer2 = finadapter.NewDraftWriter(p.FinModelRepo)
	}
	collector.add(agenttooldefs.NewEcomAssumptionSuggestionDefinition(writer2))
}

// registerRetailTools registers the retail.* surface: the operating-fact
// reads, the pulse/diagnostics/scenario/paper family and the store/equipment
// performance reads.
func registerRetailTools(collector *registerCollector, p AgentPorts) {
	// B-3：经营事实只读面。事实写入走导入管线与审批，接口只有读方法。
	if p.OperatingFacts != nil {
		collector.add(agenttooldefs.NewOperatingStoresDefinition(p.OperatingFacts))
		collector.add(agenttooldefs.NewOperatingStoreDaysDefinition(p.OperatingFacts))
		collector.add(agenttooldefs.NewStorePerformanceDefinition(p.OperatingFacts))
		collector.add(agenttooldefs.NewRentToSalesDefinition(p.OperatingFacts))
		collector.add(agenttooldefs.NewEquipmentPerformanceDefinition(p.OperatingFacts))
	}
	if p.RetailKPI != nil {
		collector.add(agenttooldefs.NewRetailOperatingPulseDefinition(p.RetailKPI))
		collector.add(agenttooldefs.NewRetailStoreDiagnosticsDefinition(p.RetailKPI))
		collector.add(agenttooldefs.NewRetailScenarioEvaluateDefinition(p.RetailKPI))
		collector.add(agenttooldefs.NewRetailPaperDefinition(p.RetailKPI))
		// B-3：store-day 指标聚合视图，复用 retailKPIRepo 的 QueryFacts 接缝。
		collector.add(agenttooldefs.NewKpiStoreDaysDefinition(p.RetailKPI))
	}
	// 电商经营读面（spec §5 定案）：站点脉搏 / 诊断 / 大促情景——读类只读，
	// 情景只评估不落库（输出顶层 data_classification=simulated）。
	if p.Ecommerce != nil {
		collector.add(agenttooldefs.NewSitePulseDefinition(p.Ecommerce))
		collector.add(agenttooldefs.NewSiteDiagnosticsDefinition(p.Ecommerce))
		collector.add(agenttooldefs.NewSiteScenarioEvaluateDefinition(p.Ecommerce))
	}
	for _, definition := range []agenttools.ToolDefinition{
		agenttooldefs.NewStoreScenarioDefinition(),
		agenttooldefs.NewEquipmentScenarioDefinition(),
	} {
		collector.add(definition)
	}
}
