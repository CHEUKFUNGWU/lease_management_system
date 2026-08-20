#!/usr/bin/env python3
"""Record the CORR-2 baseline from the OLD Python intake path (W5-2, door A).

The fixture economy is strict: inputs (document text/xlsx + the recorded LLM
JSON response the old path would have received) and the expected normalized
intake output are captured by running the ai-service producer as-is. No LLM
API key is needed — the LLM response is a fixture, and this script is the
recording instrument. W5-3's Go producer must reproduce these expected outputs
when fed the same inputs (the parity gate).

Door B (prompt fidelity) golden files are produced by rendering the four
prompt templates on a fixed input and storing the exact text; W5-3's Go
renderer must match them byte for byte.

Lifecycle (per the task instruction): this is a one-time recording tool. It
is committed under ai-service/scripts/, is not a runtime, and is deleted
together with the ai-service directory at W6-1.

Fail-loud: any scenario that fails to record aborts the run (non-zero exit) —
a missing fixture must never masquerade as a healthy baseline.
"""
import json
import sys
import os
from pathlib import Path

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "ai-service"))
os.environ.setdefault("LLM_PROVIDER", "deepseek")
os.environ.setdefault("DEEPSEEK_API_KEY", "recording-key")

import asyncio
import io
import openpyxl
from app.intake.adapters import (
    SourceMaterial,
    StoredExcelAdapter,
    _read_excel_contracts,
)
from app.intake.producer import (
    AIIntakeProducer,
    IntakeCommand,
    IntakeKind,
    _contract_prompt,
    _payment_prompt,
    _event_prompt,
    _contract_batch_prompt,
)

REPO = Path(__file__).resolve().parents[2]
BASE = REPO / "core-service" / "internal" / "agentseval" / "testdata" / "corr2"
GOLDEN = BASE / "golden"


class FixedSourceAdapter:
    def __init__(self, text, content_type):
        self._text = text
        self._ct = content_type

    async def read(self, command, max_characters):
        return SourceMaterial(text=self._text[: max_characters], content_type=self._ct)


class FixedExcelSourceAdapter:
    """Drives the real Excel adapter so deterministic_records/locators and the
    _apply_excel_evidence_safety_checks branch are actually exercised (fixes
    the '名不副实' B11 gap)."""

    def __init__(self, xlsx_bytes: bytes, content_type: str):
        self._bytes = xlsx_bytes
        self._ct = content_type

    async def read(self, command, max_characters):
        return await StoredExcelAdapter(
            lambda _bucket, _name: self._bytes
        ).read(command, max_characters)


class FixedLLMAdapter:
    """The recorded LLM response: what the upstream would have returned."""

    def __init__(self, response: str):
        self._resp = response

    async def complete(self, **options):
        return self._resp


def make_contract_xlsx(rows: list[list]):
    """Build a real xlsx whose cell dump drives the deterministic path.

    The last column carries a termination/renewal clause so the safety check
    (negation semantics) has a real line to find.
    """
    headers = ["合同编号", "承租方", "出租方", "租赁起始日", "租赁结束日", "固定租金", "币种", "条款"]
    wb = openpyxl.Workbook()
    ws = wb.active
    ws.title = "合同台账"
    ws.append(headers)
    for row in rows:
        ws.append(row)
    buf = io.BytesIO()
    wb.save(buf)
    return buf.getvalue()


def run_producer(command, source, llm_resp):
    producer = AIIntakeProducer()
    llm = FixedLLMAdapter(json.dumps(llm_resp, ensure_ascii=False)
                          if isinstance(llm_resp, (dict, list)) else llm_resp)
    result = asyncio.run(producer.produce(command, source, llm))
    payload = json.loads(result.model_dump_json())
    # intake_id is a fresh UUID per run; the baseline must be byte-reproducible,
    # so freeze it to a deterministic value derived from the case identity.
    payload["intake_id"] = "intake_frozen_" + __import__("hashlib").md5(command.kind.value.encode()).hexdigest()[:12]
    return payload


def main():
    BASE.mkdir(parents=True, exist_ok=True)
    GOLDEN.mkdir(parents=True, exist_ok=True)

    contract_text = (
        "租赁合同 编号 LEASE-CORR2-001\n"
        "承租方：示例零售有限公司（乙方）\n"
        "出租方：示例置地有限公司（甲方）\n"
        "租赁物业：示例购物中心 1 层 101 铺\n"
        "租赁起始日：2026-01-01，租赁结束日：2028-12-31\n"
        "固定租金：每月 50,000 元（不含税），每月 5 日前支付\n"
        "物业管理费：每月 3,000 元；服务费：每月 2,000 元\n"
    )
    schedule_text = (
        "租金付款计划\n"
        "| 期间起始 | 期间结束 | 应付日期 | 金额 | 付款时点 | 金额类型 |\n"
        "| 2026-01-01 | 2026-01-31 | 2026-01-01 | 50000 | 先付 | fixed_rent |\n"
        "| 2026-02-01 | 2026-02-28 | 2026-02-01 | 50000 | 先付 | fixed_rent |\n"
    )
    event_text = (
        "合同变更通知\n"
        "合同编号：LEASE-CORR2-EV-001\n"
        "鉴于经营调整，自 2026-06-01 起租赁面积由 120 平米调整为 100 平米，"
        "月固定租金由 50,000 元调整为 42,000 元。特此通知。\n"
    )
    batch_text = (
        "合同台账\n"
        "合同编号 | 承租方 | 出租方 | 租赁起始 | 租赁结束 | 月固定租金 | 币种\n"
        "L-B1 | 门店一有限公司 | 置地一号 | 2026-01-01 | 2028-12-31 | 30000 | CNY\n"
        "L-B2 | 门店二有限公司 | 置地一号 | 2026-02-01 | 2029-01-31 | 40000 | CNY\n"
    )
    XLSX_CT = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

    # ---- scenario definitions: name -> (kind, source, llm_response, contract_id) ----
    # Source is either ("text", text, content_type) or ("excel", rows, content_type).
    scenarios = []

    def add(name, kind, source, llm_resp, contract_id="", expect_error=False):
        scenarios.append((name, kind, source, llm_resp, contract_id, expect_error))

    # ---------- CONTRACT (>=10) ----------
    add("contract-full", "contract", ("text", contract_text, "application/pdf"), {
        "extracted_fields": {
            "contract_number": "LEASE-CORR2-001", "contract_name": "示例合同",
            "lessee": "示例零售有限公司", "lessor": "示例置地有限公司",
            "store_name": "示例购物中心 1 层 101 铺", "store_address": "示例市示例路 1 号",
            "commencement_date": "2026-01-01", "lease_start_date": "2026-01-01",
            "lease_end_date": "2028-12-31", "currency": "CNY", "asset_type": "real_estate",
            "area_sqm": 120.5, "fixed_rent_amount": 50000, "payment_frequency": "monthly",
            "payment_timing": "prepaid", "renewal_option": False, "termination_option": False,
            "cam_amount": 3000, "service_fee": 2000, "discount_rate_type": "incremental_borrowing_rate",
            "discount_rate": 3.5, "is_lease": True, "suggested_scope": "in_scope",
            "exemption_reason": "", "scope_confidence": 0.95,
        },
        "confidence_scores": {"contract_number": 0.99, "fixed_rent_amount": 0.98},
        "overall_confidence": 0.95, "missing_fields": [], "warnings": [],
        "evidence": [{"field": "extracted_data.contract_number", "page": 1, "quote": "LEASE-CORR2-001"}],
    })
    add("contract-no-discount-rate", "contract", ("text", contract_text, "application/pdf"), {
        "extracted_fields": {
            "contract_number": "LEASE-CORR2-002", "contract_name": "", "lessee": "示例零售有限公司",
            "lessor": "示例置地有限公司", "commencement_date": "2026-01-01",
            "lease_start_date": "2026-01-01", "lease_end_date": "2028-12-31",
            "currency": "CNY", "asset_type": "real_estate", "fixed_rent_amount": 50000,
            "payment_frequency": "monthly", "payment_timing": "prepaid",
        },
        "confidence_scores": {}, "overall_confidence": 0.9, "missing_fields": [], "warnings": [],
    })
    add("contract-no-currency", "contract", ("text", contract_text, "application/pdf"), {
        "extracted_fields": {
            "contract_number": "LEASE-CORR2-003", "lessee": "示例零售有限公司",
            "lessor": "示例置地有限公司", "commencement_date": "2026-01-01",
            "lease_start_date": "2026-01-01", "lease_end_date": "2028-12-31",
            "currency": "unknown", "asset_type": "real_estate", "fixed_rent_amount": 50000,
            "payment_timing": "prepaid",
        },
        "confidence_scores": {}, "overall_confidence": 0.85, "missing_fields": [], "warnings": [],
    })
    add("contract-missing-critical", "contract", ("text", contract_text, "application/pdf"), {
        "extracted_fields": {"contract_name": "示例合同", "currency": "CNY"},
        "confidence_scores": {}, "overall_confidence": 0.5, "missing_fields": [], "warnings": [],
    })
    add("contract-invalid-scope", "contract", ("text", contract_text, "application/pdf"), {
        "extracted_fields": {
            "contract_number": "LEASE-CORR2-005", "lessee": "示例零售有限公司",
            "lessor": "示例置地有限公司", "commencement_date": "2026-01-01",
            "lease_start_date": "2026-01-01", "lease_end_date": "2028-12-31",
            "currency": "CNY", "fixed_rent_amount": 50000, "payment_timing": "prepaid",
            "suggested_scope": "maybe_lease", "scope_confidence": 0.9,
        },
        "confidence_scores": {}, "overall_confidence": 0.9, "missing_fields": [], "warnings": [],
    })
    add("contract-short-term", "contract", ("text", contract_text, "application/pdf"), {
        "extracted_fields": {
            "contract_number": "LEASE-CORR2-008", "lessee": "示例零售有限公司",
            "lessor": "示例置地有限公司", "commencement_date": "2026-01-01",
            "lease_start_date": "2026-01-01", "lease_end_date": "2026-11-30",
            "currency": "CNY", "fixed_rent_amount": 50000, "payment_timing": "prepaid",
            "suggested_scope": "short_term_exempt", "scope_confidence": 0.92,
        },
        "confidence_scores": {}, "overall_confidence": 0.9, "missing_fields": [], "warnings": [],
    })
    add("contract-not-a-lease", "contract", ("text", contract_text, "application/pdf"), {
        "extracted_fields": {
            "contract_number": "LEASE-CORR2-009", "lessee": "示例零售有限公司",
            "lessor": "示例置地有限公司", "commencement_date": "2026-01-01",
            "lease_start_date": "2026-01-01", "lease_end_date": "2028-12-31",
            "currency": "CNY", "fixed_rent_amount": 50000, "payment_timing": "prepaid",
            "suggested_scope": "not_a_lease", "scope_confidence": 0.9, "is_lease": False,
        },
        "confidence_scores": {}, "overall_confidence": 0.9, "missing_fields": [], "warnings": [],
    })
    add("contract-low-scope-confidence", "contract", ("text", contract_text, "application/pdf"), {
        "extracted_fields": {
            "contract_number": "LEASE-CORR2-006", "lessee": "示例零售有限公司",
            "lessor": "示例置地有限公司", "commencement_date": "2026-01-01",
            "lease_start_date": "2026-01-01", "lease_end_date": "2028-12-31",
            "currency": "CNY", "fixed_rent_amount": 50000, "payment_timing": "prepaid",
            "suggested_scope": "in_scope", "scope_confidence": 0.6,
        },
        "confidence_scores": {}, "overall_confidence": 0.9, "missing_fields": [], "warnings": [],
    })
    add("contract-unsafe-confidence", "contract", ("text", contract_text, "application/pdf"), {
        "extracted_fields": {
            "contract_number": "LEASE-CORR2-007", "lessee": "示例零售有限公司",
            "lessor": "示例置地有限公司", "commencement_date": "2026-01-01",
            "lease_start_date": "2026-01-01", "lease_end_date": "2028-12-31",
            "currency": "CNY", "fixed_rent_amount": 50000, "payment_timing": "prepaid",
            "suggested_scope": "in_scope", "scope_confidence": 0.95,
        },
        "confidence_scores": {"contract_number": 150.0, "fixed_rent_amount": -3.0},
        "overall_confidence": 99.0, "missing_fields": [], "warnings": [],
    })
    add("contract-asset-type-cn", "contract", ("text", contract_text, "application/pdf"), {
        "extracted_fields": {
            "contract_number": "LEASE-CORR2-010", "lessee": "示例零售有限公司",
            "lessor": "示例置地有限公司", "commencement_date": "2026-01-01",
            "lease_start_date": "2026-01-01", "lease_end_date": "2028-12-31",
            "currency": "CNY", "asset_type": "商业物业", "fixed_rent_amount": 50000,
            "payment_timing": "月度支付",
        },
        "confidence_scores": {}, "overall_confidence": 0.9, "missing_fields": [], "warnings": [],
    })
    add("contract-both-missing", "contract", ("text", contract_text, "application/pdf"), {
        "extracted_fields": {
            "contract_number": "LEASE-CORR2-012", "lessee": "示例零售有限公司",
            "lessor": "示例置地有限公司", "fixed_rent_amount": 50000, "payment_timing": "prepaid",
        },
        "confidence_scores": {}, "overall_confidence": 0.8, "missing_fields": [], "warnings": [],
    })
    # Negative evidence: the LLM proposes a quote that does not exist in the source.
    # _evidence_quote_matches must reject it -> evidence not complete (底3 来源追溯).
    add("contract-negative-evidence", "contract", ("text", contract_text, "application/pdf"), {
        "extracted_fields": {
            "contract_number": "LEASE-CORR2-013", "lessee": "示例零售有限公司",
            "lessor": "示例置地有限公司", "commencement_date": "2026-01-01",
            "lease_start_date": "2026-01-01", "lease_end_date": "2028-12-31",
            "currency": "CNY", "fixed_rent_amount": 50000, "payment_timing": "prepaid",
            "suggested_scope": "in_scope", "scope_confidence": 0.95,
        },
        "confidence_scores": {}, "overall_confidence": 0.9, "missing_fields": [], "warnings": [],
        "evidence": [{"field": "extracted_data.contract_number", "page": 1, "quote": "押金三倍赔偿条款"}],
    })

    # ---------- PAYMENT_SCHEDULE (>=10) ----------
    base_schedule = {
        "schedules": [
            {"period_start": "2026-01-01", "period_end": "2026-01-31", "due_date": "2026-01-01",
             "amount": 50000, "payment_timing": "prepaid", "is_fixed": True, "is_lease_component": True,
             "amount_type": "fixed_rent", "currency": "CNY", "confidence": 0.95},
            {"period_start": "2026-02-01", "period_end": "2026-02-28", "due_date": "2026-02-01",
             "amount": 50000, "payment_timing": "prepaid", "is_fixed": True, "is_lease_component": True,
             "amount_type": "fixed_rent", "currency": "CNY", "confidence": 0.95},
        ],
        "overall_confidence": 0.95, "missing_fields": [], "warnings": [], "total_schedules": 2,
    }
    add("payment-full", "payment_schedule", ("text", schedule_text, "application/pdf"), base_schedule)
    p2 = json.loads(json.dumps(base_schedule))
    p2["schedules"].append({"period_start": "2026-03-01", "period_end": "2026-03-31", "due_date": "", "amount": 0})
    add("payment-skip-invalid", "payment_schedule", ("text", schedule_text, "application/pdf"), p2)
    p3 = json.loads(json.dumps(base_schedule))
    p3["schedules"][0]["amount_type"] = "turnover_rent"
    p3["schedules"][0]["is_fixed"] = False
    add("payment-turnover-rent", "payment_schedule", ("text", schedule_text, "application/pdf"), p3)
    p4 = json.loads(json.dumps(base_schedule))
    p4["schedules"][0]["payment_timing"] = ""
    add("payment-missing-timing", "payment_schedule", ("text", schedule_text, "application/pdf"), p4)
    p5 = json.loads(json.dumps(base_schedule))
    p5["schedules"][0]["due_date"] = "2026/01/01"
    add("payment-bad-date-format", "payment_schedule", ("text", schedule_text, "application/pdf"), p5)
    p6 = json.loads(json.dumps(base_schedule))
    p6["schedules"][0]["amount"] = "五万"
    add("payment-non-numeric-amount", "payment_schedule", ("text", schedule_text, "application/pdf"), p6)
    p7 = json.loads(json.dumps(base_schedule))
    p7["schedules"][0]["amount"] = -100
    add("payment-negative-amount", "payment_schedule", ("text", schedule_text, "application/pdf"), p7)
    p8 = json.loads(json.dumps(base_schedule))
    p8["schedules"][1]["payment_timing"] = "postpaid"
    add("payment-mixed-timing", "payment_schedule", ("text", schedule_text, "application/pdf"), p8)
    p9 = json.loads(json.dumps(base_schedule))
    p9["schedules"][0]["confidence"] = 0.5
    add("payment-low-confidence", "payment_schedule", ("text", schedule_text, "application/pdf"), p9)
    add("payment-llm-fallback", "payment_schedule", ("text", schedule_text, "application/pdf"), "not json at all")
    add("payment-fallback-no-header", "payment_schedule",
        ("text", "租金明细\n日期/金额\n2026-01-01/50000\n", "application/pdf"), "not json at all")
    p12 = json.loads(json.dumps(base_schedule))
    p12["schedules"][0]["currency"] = ""
    add("payment-no-currency", "payment_schedule", ("text", schedule_text, "application/pdf"), p12)
    # Negative evidence for payment schedule
    p13 = json.loads(json.dumps(base_schedule))
    p13["evidence"] = [{"field": "schedules[0].amount", "page": 1, "quote": "押金两倍"}]
    add("payment-negative-evidence", "payment_schedule", ("text", schedule_text, "application/pdf"), p13)

    # ---------- EVENT (>=10) ----------
    def ev(**o):
        base = {
            "event": {
                "contract_id": "c-001", "contract_number": "LEASE-CORR2-EV-001",
                "event_type": "area_adjustment", "effective_date": "2026-06-01",
                "original_value": "120", "new_value": "100", "change_reason": "经营调整",
                "judgment_basis": "通知明确载明面积调整",
                "revision_parameters": {"new_area_sqm": 100, "new_monthly_rent": 42000},
                "field_confidence": {"event_type": 0.95, "effective_date": 0.95},
            },
            "overall_confidence": 0.9, "missing_fields": [], "warnings": [],
            "evidence": [{"field": "event.event_type", "page": 1, "quote": "面积由 120 平米调整为 100 平米"}],
        }
        base.update(o)
        return base
    add("event-modification", "event", ("text", event_text, "application/pdf"), ev())
    add("event-missing-type", "event", ("text", event_text, "application/pdf"),
        ev(event={"contract_id": "c-001", "contract_number": "LEASE-CORR2-EV-001", "effective_date": "2026-06-01", "change_reason": "调整", "judgment_basis": "通知"}, missing_fields=["event_type"]))
    add("event-unstandardized-type", "event", ("text", event_text, "application/pdf"),
        ev(event={"contract_id": "c-001", "contract_number": "LEASE-CORR2-EV-001", "event_type": "renovation", "effective_date": "2026-06-01", "change_reason": "装修", "judgment_basis": "通知"}, missing_fields=["event_type_review"]))
    add("event-no-revision-params", "event", ("text", event_text, "application/pdf"),
        ev(event={"contract_id": "c-001", "contract_number": "LEASE-CORR2-EV-001", "event_type": "rent_change", "effective_date": "2026-06-01", "change_reason": "租金调整", "judgment_basis": "通知"}, revision_parameters={}))
    add("event-low-overall", "event", ("text", event_text, "application/pdf"), ev(overall_confidence=0.4))
    add("event-empty-llm", "event", ("text", event_text, "application/pdf"), "not json at all")
    add("event-early-termination", "event", ("text", event_text, "application/pdf"),
        ev(event={"contract_id": "c-001", "contract_number": "LEASE-CORR2-EV-001", "event_type": "early_termination", "effective_date": "2026-08-31", "change_reason": "闭店", "judgment_basis": "闭店通知", "revision_parameters": {}}))
    add("event-reassessment", "event", ("text", event_text, "application/pdf"),
        ev(event={"contract_id": "c-001", "contract_number": "LEASE-CORR2-EV-001", "event_type": "reassessment", "effective_date": "2026-06-01", "change_reason": "续租选择权重估", "judgment_basis": "续租意图变化", "revision_parameters": {"new_lease_end_date": "2030-12-31"}}))
    add("event-no-contract-id", "event", ("text", event_text, "application/pdf"),
        ev(event={"contract_number": "LEASE-CORR2-EV-001", "event_type": "rent_change", "effective_date": "2026-06-01", "change_reason": "xx", "judgment_basis": "yy"}, missing_fields=["contract_id", "change_reason", "judgment_basis"]))
    add("event-index-update", "event", ("text", event_text, "application/pdf"),
        ev(event={"contract_id": "c-001", "contract_number": "LEASE-CORR2-EV-001", "event_type": "index_update", "effective_date": "2026-07-01", "change_reason": "CPI 更新", "judgment_basis": "指数条款", "revision_parameters": {"index_name": "CPI", "index_value": "3.2"}}))
    # Negative evidence for event
    add("event-negative-evidence", "event", ("text", event_text, "application/pdf"),
        ev(evidence=[{"field": "event.event_type", "page": 1, "quote": "租金上调至一百万元"}]))

    # ---------- CONTRACT_BATCH (>=10) ----------
    batch_llm = {
        "contracts": [
            {"contract_number": "L-B1", "lessee": "门店一有限公司", "lessor": "置地一号",
             "commencement_date": "2026-01-01", "lease_start_date": "2026-01-01", "lease_end_date": "2028-12-31",
             "currency": "CNY", "fixed_rent_amount": 30000, "payment_timing": "postpaid",
             "suggested_scope": "in_scope", "scope_confidence": 0.95,
             "discount_rate_type": "incremental_borrowing_rate", "discount_rate": 3.5},
            {"contract_number": "L-B2", "lessee": "门店二有限公司", "lessor": "置地一号",
             "commencement_date": "2026-02-01", "lease_start_date": "2026-02-01", "lease_end_date": "2029-01-31",
             "currency": "CNY", "fixed_rent_amount": 40000, "payment_timing": "postpaid",
             "suggested_scope": "in_scope", "scope_confidence": 0.95},
            {"contract_number": "L-B3", "lessee": "门店三有限公司", "lessor": "置地二号",
             "commencement_date": "2026-03-01", "lease_start_date": "2026-03-01", "lease_end_date": "2026-12-31",
             "currency": "CNY", "fixed_rent_amount": 20000, "payment_timing": "postpaid",
             "suggested_scope": "short_term_exempt", "scope_confidence": 0.93},
        ],
        "total_count": 3, "overall_confidence": 0.93, "missing_fields": [], "warnings": [],
    }
    contract_xlsx_bytes = make_contract_xlsx([
        ["L-E1", "门店一有限公司", "置地一号", "2026-01-01", "2028-12-31", 30000, "CNY",
         "合同终止选择权：不行使；续租选择权：合同到期后不会续租"],
        ["L-E2", "门店二有限公司", "置地一号", "2026-02-01", "2029-01-31", 40000, "CNY", ""],
    ])
    add("batch-full", "contract_batch", ("text", batch_text, "application/pdf"), batch_llm)
    b2 = json.loads(json.dumps(batch_llm))
    b2["contracts"][0].pop("currency")
    add("batch-missing-currency", "contract_batch", ("text", batch_text, "application/pdf"), b2)
    b3 = json.loads(json.dumps(batch_llm))
    b3["contracts"][0].pop("discount_rate_type")
    b3["contracts"][0].pop("discount_rate")
    add("batch-missing-discount", "contract_batch", ("text", batch_text, "application/pdf"), b3)
    b4 = json.loads(json.dumps(batch_llm))
    b4["contracts"].append({"contract_number": "L-B4", "lessee": "门店四", "lessor": "置地三号",
        "commencement_date": "2026-04-01", "lease_start_date": "2026-04-01", "lease_end_date": "2026-12-31", "currency": "CNY"})
    add("batch-skip-missing-critical", "contract_batch", ("text", batch_text, "application/pdf"), b4)
    b5 = json.loads(json.dumps(batch_llm))
    b5["contracts"][0]["scope_confidence"] = 0.5
    b5["contracts"][0]["suggested_scope"] = "not_a_lease"
    add("batch-low-scope-confidence", "contract_batch", ("text", batch_text, "application/pdf"), b5)
    add("batch-empty", "contract_batch", ("text", batch_text, "application/pdf"),
        {"contracts": [], "total_count": 0, "overall_confidence": 0.0, "missing_fields": [], "warnings": []})
    b7 = json.loads(json.dumps(batch_llm))
    b7["contracts"] = [b7["contracts"][0]]
    b7["contracts"][0]["fixed_rent_amount"] = "-30000"
    add("batch-negative-amount", "contract_batch", ("text", batch_text, "application/pdf"), b7, expect_error=True)
    b8 = json.loads(json.dumps(batch_llm))
    b8["contracts"][0].pop("lessee")
    add("batch-skip-no-lessee", "contract_batch", ("text", batch_text, "application/pdf"), b8)
    b9 = json.loads(json.dumps(batch_llm))
    b9["contracts"][0]["renewal_option"] = True
    b9["contracts"][0]["termination_option"] = True
    add("batch-negation", "contract_batch", ("text", batch_text, "application/pdf"), b9)
    b10 = json.loads(json.dumps(batch_llm))
    b10["contracts"][1]["confidence"] = 0.4
    b10["contracts"][2]["confidence"] = 0.9
    add("batch-mixed-confidence", "contract_batch", ("text", batch_text, "application/pdf"), b10)
    # Excel deterministic fallback: REAL xlsx so _apply_excel_evidence_safety_checks
    # and the ledger scope_source path are actually exercised (B11 名实相符).
    add("batch-excel-deterministic-fallback", "contract_batch",
        ("excel", contract_xlsx_bytes, XLSX_CT), {"contracts": [], "total_count": 0, "overall_confidence": 0.0})
    # Negative evidence for batch
    b14 = json.loads(json.dumps(batch_llm))
    b14["evidence"] = [{"field": "contracts[0].contract_number", "page": 1, "quote": "虚构合同编号 X-999"}]
    add("batch-negative-evidence", "contract_batch", ("text", batch_text, "application/pdf"), b14)

    # Sanity: every kind has >= 10 cases.
    from collections import Counter
    counts = Counter(kind for _, kind, _, _, _, _ in scenarios)
    for kind in ("contract", "payment_schedule", "event", "contract_batch"):
        if counts[kind] < 10:
            print(f"FATAL: kind {kind} has only {counts[kind]} scenarios (<10); refusing to write an incomplete baseline")
            sys.exit(1)

    cases = []
    failures = []
    for name, kind, source, llm_resp, contract_id, expect_error in scenarios:
        file_id = f"file-{name}"
        object_name = f"{name}.pdf"
        llm_wire = llm_resp if isinstance(llm_resp, str) else json.dumps(llm_resp, ensure_ascii=False)
        if source[0] == "text":
            _, text, ct = source
            source_adapter = FixedSourceAdapter(text, ct)
            input_payload = {"text": text, "content_type": ct, "contract_id": contract_id, "llm_response": llm_wire}
        else:
            _, xlsx_bytes, ct = source
            # The xlsx embeds timestamps, so its bytes are not reproducible
            # across runs. Store it as a committed asset once and reuse the
            # committed bytes thereafter — the baseline stays byte-identical
            # and the committed file is the single source of truth.
            xlsx_asset = BASE / "assets" / f"{name}.xlsx"
            xlsx_asset.parent.mkdir(parents=True, exist_ok=True)
            if xlsx_asset.exists():
                xlsx_bytes = xlsx_asset.read_bytes()
            else:
                xlsx_asset.write_bytes(xlsx_bytes)
            source_adapter = FixedExcelSourceAdapter(xlsx_bytes, ct)
            object_name = f"{name}.xlsx"
            input_payload = {"xlsx_base64": __import__("base64").b64encode(xlsx_bytes).decode(),
                             "content_type": ct, "contract_id": contract_id, "llm_response": llm_wire}
        command = IntakeCommand(kind=IntakeKind(kind), file_id=file_id, object_name=object_name,
                                content_type=source[2] if source[0] == "text" else source[2],
                                contract_id=contract_id)
        try:
            expected = run_producer(command, source_adapter, llm_resp)
        except Exception as exc:
            if expect_error:
                expected = {"error": f"{type(exc).__name__}: {exc}"}
            else:
                failures.append(f"{name}: {type(exc).__name__}: {exc}")
                continue
        if expect_error and "error" not in expected:
            failures.append(f"{name}: expected a negative-amount rejection but the producer returned an output")
            continue
        case = {"name": name, "kind": kind, "input": input_payload, "expected": expected}
        (BASE / f"{name}.json").write_text(json.dumps(case, ensure_ascii=False, indent=2), encoding="utf-8")
        cases.append(name)

    if failures:
        print("FAIL-LOUD: the following scenarios failed to record and NO baseline is complete:")
        for f in failures:
            print("  -", f)
        sys.exit(1)

    manifest = {"schema_version": "corr2.v1", "producer": "ai-service/app/intake/producer.py",
                "defined": len(scenarios), "recorded": len(cases), "cases": sorted(cases)}
    (BASE / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"recorded {len(cases)}/{len(scenarios)} cases -> {BASE}")

    # ---------- Door B: prompt golden files ----------
    prompt_text = "租赁合同 编号 GOLDEN-001\n承租方：示例公司\n出租方：示例置地\n租赁起始日：2026-01-01\n"
    goldens = {
        "prompt-contract": _contract_prompt(prompt_text),
        "prompt-payment_schedule": _payment_prompt(prompt_text),
        "prompt-event": _event_prompt(prompt_text, "c-001"),
        "prompt-contract_batch": _contract_batch_prompt(prompt_text),
    }
    for name, rendered in goldens.items():
        (GOLDEN / f"{name}.golden.txt").write_text(rendered, encoding="utf-8")
        print(f"recorded golden prompt {name} ({len(rendered)} chars)")
    # A small reader note keeps the golden dir self-describing.
    (GOLDEN / "README.md").write_text(
        "# CORR-2 prompt golden files (door B)\n\n"
        "Recorded by ai-service/scripts/record_corr2_baseline.py from the OLD Python\n"
        "prompt templates (producer.py:508-702) on a fixed input. W5-3's Go renderer\n"
        "must reproduce these byte-for-byte; any wording change must make the parity\n"
        "test red and be justified in the PR.\n", encoding="utf-8")


if __name__ == "__main__":
    main()
