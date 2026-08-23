package tools

// fpna.store_pnl.read 是单店利润表引擎（internal/storepnl，SM3）的只读 Agent
// 出口。HTTP 面（GET /stores/:id/pnl）与 Agent 共用同一个投影核心，这里只做
// 参数校验、期间解析（S1-2 retailperiod）、法人范围回填与错误映射——
// 不对 KPI 做任何重算，也不落任何业务表。
//
// 口径纪律（AGENTS.md）：经营占用行算基本租金 + 服务费 + 当期变量租金；
// IFRS 16 块由租赁端口读出，两套口径并列呈现、不可互相替代；decision_ready
// 为 false 时行值保持缺失（nil），绝不填 0、绝不反推事实。

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/services/retailperiod"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

// StorePnlReader is the application seam the read tool consumes. The
// production implementation lives in handlers (the only place with the S1
// port adapters); this interface keeps the Agent Runtime free of repository
// and gin dependencies. LegalEntityID inside q.Ref must be filled by the
// caller from the authenticated principal — the port never guesses one.
type StorePnlReader interface {
	Project(ctx context.Context, q StorePnlQuery) (*storepnl.StorePnl, error)
}

// StorePnlQuery is the fully resolved projection request: the caller
// (this tool) has already parsed and validated the period and the plan
// version column; the seam only gates scope and projects.
type StorePnlQuery struct {
	Ref           storepnl.StoreRef
	Period        storepnl.Period
	Pair          [2]storepnl.ColumnRef
	Basis         storepnl.BasisMode
	PlanVersionID string
}

// StorePnlReadArguments mirrors the HTTP projection surface but tightened
// for the agent: either a retailperiod spec (period) or the legacy
// as_of+window_days anchor; data_classification follows the retail
// discipline (simulated ⇒ dataset_version required, production ⇒ forbidden).
type StorePnlReadArguments struct {
	StoreID        string `json:"store_id"`
	Period         string `json:"period,omitempty"`
	AsOf           string `json:"as_of,omitempty"`
	WindowDays     int    `json:"window_days,omitempty"`
	DataClass      string `json:"data_classification"`
	DatasetVersion string `json:"dataset_version,omitempty"`
	SourceSystem   string `json:"source_system,omitempty"`
	Basis          string `json:"basis,omitempty"`
	Primary        string `json:"primary,omitempty"`
	Secondary      string `json:"secondary,omitempty"`
	PlanVersionID  string `json:"plan_version_id,omitempty"`
}

// StorePnlToolData wraps the projection with the numeric-authority and
// side-effect contract every deterministic read tool carries.
type StorePnlToolData struct {
	*storepnl.StorePnl
	NumericAuthority string `json:"numeric_authority"`
	SideEffects      bool   `json:"side_effects"`
}

// NewStorePnlReadDefinition registers the store profit-and-loss read tool.
// P0-8: a nil reader (port not wired) keeps the tool honest — it refuses
// with "unavailable" instead of fabricating a projection; the wiring never
// registers the nil version unconditionally over a real port.
func NewStorePnlReadDefinition(reader StorePnlReader) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name:        "fpna.store_pnl.read",
			Version:     "v1",
			DisplayName: "读取单店利润表",
			Description: "读取单店利润表投影（经营/IFRS 16 双口径并排、决策就绪状态与缺口）；数字全部来自确定性服务，只读不写。口径：经营占用 = 基本租金 + 服务费 + 当期变量租金，与 IFRS 16 口径并列维护、不可互相替代。",
			Level:       agenttools.LevelRead,
			ReadOnly:    true,
			Permissions: []agenttools.Permission{{Resource: "reports", Action: "read"}},
			InputSchema: json.RawMessage(`{
				"type":"object",
				"additionalProperties":false,
				"required":["store_id","data_classification"],
				"properties":{
					"store_id":{"type":"string","format":"uuid"},
					"period":{"type":"string","description":"retailperiod 规格：YYYY-MM、YYYY-Qn、YYYY、YYYY-Wnn、last-month、this-quarter、或 rolling 天数（1-365）。给出 period 时忽略 as_of/window_days。"},
					"as_of":{"type":"string","pattern":"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"},
					"window_days":{"type":"integer","minimum":7,"maximum":28},
					"data_classification":{"type":"string","enum":["production","simulated"]},
					"dataset_version":{"type":"string"},
					"source_system":{"type":"string"},
					"basis":{"type":"string","enum":["operating","ifrs16","side_by_side"]},
					"primary":{"type":"string","enum":["actual","prior_year","budget","forecast"]},
					"secondary":{"type":"string","enum":["actual","prior_year","budget","forecast"]},
					"plan_version_id":{"type":"string"}
				}
			}`),
			SupportsDryRun: true,
			MaxRows:        2000,
			TimeoutSeconds: 20,
		},
		SkillIDs: []string{"fpna_copilot"},
		Handler: func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
			if reader == nil {
				return rejected(call.CallID, agenttools.ErrorDataUnavailable, "store pnl reader unavailable"), nil
			}
			execution, err := agenttools.RequireExecutionContext(ctx)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorUnauthenticated, "authenticated tool context is required"), nil
			}
			legalEntityID := strings.TrimSpace(execution.Principal.Scope.LegalEntityID)
			if legalEntityID == "" {
				return rejected(call.CallID, agenttools.ErrorScopeDenied, "legal entity scope is required"), nil
			}
			args, err := decodeStorePnlReadArgs(call.Arguments)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, err.Error()), nil
			}
			query, err := buildStorePnlQuery(args, legalEntityID)
			if err != nil {
				return rejected(call.CallID, agenttools.ErrorInvalidArguments, err.Error()), nil
			}
			pnl, err := reader.Project(ctx, query)
			if err != nil {
				return rejected(call.CallID, storePnlErrorCode(err), storePnlPublicError(err)), nil
			}
			sources := storePnlSources(pnl)
			return agenttools.ToolResult{
				CallID:  call.CallID,
				Status:  agenttools.StatusCompleted,
				Data:    StorePnlToolData{StorePnl: pnl, NumericAuthority: "deterministic_service", SideEffects: false},
				Sources: sources,
			}, nil
		},
	}
}

// decodeStorePnlReadArgs applies strict decoding: unknown fields reject the
// whole call so a later API change never silently truncates the request.
func decodeStorePnlReadArgs(raw json.RawMessage) (StorePnlReadArguments, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return StorePnlReadArguments{}, errors.New("arguments are required")
	}
	args, err := decodeStrict[StorePnlReadArguments](raw)
	if err != nil {
		return StorePnlReadArguments{}, errors.New("arguments contain unsupported or invalid fields")
	}
	return args, nil
}

func buildStorePnlQuery(args StorePnlReadArguments, legalEntityID string) (StorePnlQuery, error) {
	storeID, err := parseStoreID(args.StoreID)
	if err != nil {
		return StorePnlQuery{}, err
	}
	classification := strings.TrimSpace(args.DataClass)
	if classification != "production" && classification != "simulated" {
		return StorePnlQuery{}, errors.New("data_classification must be production or simulated")
	}
	dataset := strings.TrimSpace(args.DatasetVersion)
	if (classification == "simulated") != (dataset != "") {
		return StorePnlQuery{}, errors.New("dataset_version is required for simulated and forbidden for production")
	}
	basis := storepnl.BasisMode(strings.TrimSpace(args.Basis))
	if basis == "" {
		basis = storepnl.BasisSideBySide
	}
	if basis != storepnl.BasisOperating && basis != storepnl.BasisIFRS16 && basis != storepnl.BasisSideBySide {
		return StorePnlQuery{}, errors.New("basis must be operating, ifrs16 or side_by_side")
	}
	primary := columnRefOrDefault(args.Primary, storepnl.ColActual)
	secondary := columnRefOrDefault(args.Secondary, storepnl.ColBudget)
	if primary == secondary {
		return StorePnlQuery{}, errors.New("primary and secondary columns must differ")
	}

	ref := storepnl.StoreRef{
		StoreID: storeID, LegalEntityID: legalEntityID,
		Classification: classification, DatasetVersion: dataset,
		SourceSystem: strings.TrimSpace(args.SourceSystem),
	}

	periodSpec := strings.TrimSpace(args.Period)
	if periodSpec != "" {
		anchor := time.Now()
		if asOf := strings.TrimSpace(args.AsOf); asOf != "" {
			parsed, err := time.Parse("2006-01-02", asOf)
			if err != nil {
				return StorePnlQuery{}, errors.New("as_of must be YYYY-MM-DD")
			}
			anchor = parsed
		}
		window, err := retailperiod.Parse(periodSpec, anchor)
		if err != nil {
			return StorePnlQuery{}, err
		}
		ref.DateFrom = window.From.Format("2006-01-02")
		ref.DateTo = window.To.Format("2006-01-02")
		ref.PeriodLabel = window.Label
		ref.PeriodKind = string(window.Period.Kind)
		ref.AsOf = window.To.Format("2006-01-02")
		return StorePnlQuery{
			Ref: ref, Period: storepnl.Period{From: ref.DateFrom, To: ref.DateTo},
			Pair: [2]storepnl.ColumnRef{primary, secondary}, Basis: basis,
			PlanVersionID: strings.TrimSpace(args.PlanVersionID),
		}, nil
	}

	asOf := strings.TrimSpace(args.AsOf)
	if asOf == "" {
		return StorePnlQuery{}, errors.New("either period or as_of is required")
	}
	parsedAsOf, err := time.Parse("2006-01-02", asOf)
	if err != nil {
		return StorePnlQuery{}, errors.New("as_of must be YYYY-MM-DD")
	}
	if args.WindowDays != 7 && args.WindowDays != 14 && args.WindowDays != 28 {
		return StorePnlQuery{}, errors.New("window_days must be 7, 14 or 28")
	}
	ref.AsOf = asOf
	ref.WindowDays = args.WindowDays
	from := parsedAsOf.AddDate(0, 0, -(args.WindowDays - 1))
	return StorePnlQuery{
		Ref: ref,
		Period: storepnl.Period{
			From: from.Format("2006-01-02"), To: parsedAsOf.Format("2006-01-02"),
		},
		Pair: [2]storepnl.ColumnRef{primary, secondary}, Basis: basis,
		PlanVersionID: strings.TrimSpace(args.PlanVersionID),
	}, nil
}

func columnRefOrDefault(raw string, fallback storepnl.ColumnRef) storepnl.ColumnRef {
	switch storepnl.ColumnRef(strings.TrimSpace(raw)) {
	case storepnl.ColActual, storepnl.ColPriorYear, storepnl.ColBudget, storepnl.ColForecast:
		return storepnl.ColumnRef(strings.TrimSpace(raw))
	}
	return fallback
}

// storePnlErrorCode maps the seam's errors preserving scope_denied reasons
// (底线 1): a scope denial must never be softened into "no data".
func storePnlErrorCode(err error) agenttools.ErrorCode {
	if errors.Is(err, errStoreScopeDenied) || errors.Is(err, errStoreNotFound) {
		return agenttools.ErrorScopeDenied
	}
	if errors.Is(err, errStoreMasterDataUnavailable) {
		return agenttools.ErrorDataUnavailable
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "required") || strings.Contains(message, "must") || strings.Contains(message, "uuid") || strings.Contains(message, "yyyy") || strings.Contains(message, "invalid") {
		return agenttools.ErrorInvalidArguments
	}
	return agenttools.ErrorBusinessFailure
}

func storePnlPublicError(err error) string {
	switch {
	case errors.Is(err, errStoreNotFound):
		return "scope_denied: store is outside the caller's scope"
	case errors.Is(err, errStoreScopeDenied):
		return err.Error()
	case errors.Is(err, errStoreMasterDataUnavailable):
		return "store pnl reader unavailable"
	default:
		return strings.TrimSpace(err.Error())
	}
}

func storePnlSources(pnl *storepnl.StorePnl) []agenttools.ToolSource {
	if pnl == nil {
		return nil
	}
	q := url.Values{}
	q.Set("data_classification", pnl.Classification)
	if pnl.DatasetVersion != "" {
		q.Set("dataset_version", pnl.DatasetVersion)
	}
	if len(pnl.Period.From) >= 7 {
		q.Set("period", pnl.Period.From[:7])
	}
	link := sourceURL("/stores/"+pnl.StoreID+"/pnl", q)
	return []agenttools.ToolSource{{
		Type: "store_pnl", ID: pnl.StoreID, Title: "单店利润表投影",
		Locator: link, URL: link,
		Classification: pnl.Classification, DatasetVersion: pnl.DatasetVersion, AsOf: pnl.AsOf,
	}}
}

// Sentinel errors shared with the handler-side gate (store_pnl_agent.go).
// scope_denied 原因保留，不得软化成「无数据」（底线 1）。
var (
	errStoreScopeDenied           = errors.New("scope_denied: store outside caller data scope")
	errStoreNotFound              = errors.New("store not found")
	errStoreMasterDataUnavailable = errors.New("store master data reader unavailable")
)

// The three exported constructors let the handler-side gate signal the exact
// sentinels this tool maps (storePnlErrorCode), so the error classification
// lives in one place and a cross-tenant read can never leak as a business
// failure. Usage:
//   - ErrStoreScopeDenied: store outside caller dimension scope → scope_denied
//   - ErrStoreNotFound: unknown store OR wrong tenant (no existence leak) → scope_denied wording
//   - ErrStoreMasterDataUnavailable: master-data port not wired → unavailable
func ErrStoreScopeDenied() error          { return errStoreScopeDenied }
func ErrStoreNotFound() error             { return errStoreNotFound }
func ErrStoreMasterDataUnavailable() error { return errStoreMasterDataUnavailable }
