"""Deep producer for versioned AI intake drafts.

Source and LLM adapters supply raw facts. This module owns normalization,
evidence truth, confidence, and the mandatory Assist Mode review gate.
"""

from dataclasses import dataclass
from enum import Enum
import json
from typing import Any

from app.intake.adapters import (
    IntakeAdapterError,
    LLMAdapter,
    SourceAdapter,
    SourceMaterial,
)
from app.intake.models import (
    ContractBatchIntakeResponse,
    ContractDraftData,
    ContractIntakeResponse,
    IntakeResponse,
    PaymentScheduleIntakeResponse,
    PaymentScheduleItem,
    build_contract_intake,
    build_contract_batch_intake,
    build_payment_schedule_intake,
)


class IntakeKind(str, Enum):
    CONTRACT = "contract"
    PAYMENT_SCHEDULE = "payment_schedule"
    CONTRACT_BATCH = "contract_batch"


@dataclass(frozen=True)
class IntakeCommand:
    kind: IntakeKind
    file_id: str
    object_name: str
    content_type: str
    mode: str = "assist"


class IntakeProducerError(Exception):
    def __init__(self, detail: str, status_code: int = 500):
        super().__init__(detail)
        self.detail = detail
        self.status_code = status_code


class AIIntakeProducer:
    async def produce(
        self,
        command: IntakeCommand,
        source_adapter: SourceAdapter,
        llm_adapter: LLMAdapter,
    ) -> IntakeResponse:
        if command.mode != "assist":
            raise IntakeProducerError(
                "当前仅支持 Assist Mode。Auto-Post Mode 需另行配置", 400
            )
        try:
            material = await source_adapter.read(
                command, self._max_characters(command.kind)
            )
        except IntakeAdapterError as exc:
            raise IntakeProducerError(str(exc)) from exc
        if command.kind == IntakeKind.CONTRACT:
            return await self._produce_contract(command, material.text, llm_adapter)
        if command.kind == IntakeKind.PAYMENT_SCHEDULE:
            return await self._produce_payment(command, material.text, llm_adapter)
        if command.kind == IntakeKind.CONTRACT_BATCH:
            return await self._produce_contract_batch(command, material, llm_adapter)
        raise IntakeProducerError(f"unsupported intake kind: {command.kind}", 400)

    @staticmethod
    def _max_characters(kind: IntakeKind) -> int:
        return 30000 if kind == IntakeKind.CONTRACT_BATCH else 15000

    async def _produce_contract(
        self, command: IntakeCommand, file_content: str, llm_adapter: LLMAdapter
    ) -> ContractIntakeResponse:
        try:
            content = await llm_adapter.complete(
                system=(
                    "你是一位专业的 IFRS 16 租赁合同解析专家。请准确提取合同字段。"
                    "如果字段未在合同中出现，不要猜测，标记为缺失。"
                ),
                prompt=_contract_prompt(file_content),
                temperature=0.1,
                max_tokens=2500,
                response_format={"type": "json_object"},
            )
            try:
                parsed = json.loads(content)
            except (TypeError, json.JSONDecodeError):
                parsed = {
                    "extracted_fields": {},
                    "confidence_scores": {},
                    "overall_confidence": 0.5,
                    "missing_fields": [],
                    "warnings": ["解析响应格式异常，请人工检查"],
                }

            extracted = dict(parsed.get("extracted_fields") or {})
            confidence = _sanitize_confidence_scores(
                parsed.get("confidence_scores") or {}
            )
            confidence["overall"] = _sanitize_confidence(
                parsed.get("overall_confidence")
            )
            discount_missing, discount_warnings = _check_discount_rate_missing(
                extracted
            )
            currency_missing, currency_warnings = _check_currency_missing(extracted)
            missing_fields, field_warnings = _check_critical_fields(extracted)
            _, scope_warnings = _normalize_lease_scope(extracted)
            extracted["asset_type"] = _normalize_asset_type(extracted.get("asset_type"))
            if extracted.get("payment_timing"):
                extracted["payment_timing"] = _normalize_payment_timing(
                    extracted.get("payment_timing")
                )

            warnings = (
                discount_warnings
                + currency_warnings
                + field_warnings
                + scope_warnings
                + _as_list(parsed.get("warnings"))
                + ["AI Assist Mode: 合同草稿需人工确认后入库"]
            )
            missing = sorted(
                set(
                    missing_fields
                    + _as_list(parsed.get("missing_fields"))
                    + (["discount_rate"] if discount_missing else [])
                    + (["currency"] if currency_missing else [])
                )
            )
            normalized = _normalize_contract_record(
                extracted, default_payment_frequency=""
            )
            normalized["confidence"] = _sanitize_confidence(
                extracted.get("confidence", confidence["overall"])
            )
            return build_contract_intake(
                file_id=command.file_id,
                object_name=command.object_name,
                content_type=command.content_type,
                extracted_data=normalized,
                confidence_scores=confidence,
                missing_fields=missing,
                warnings=warnings,
                evidence_missing_reason=(
                    "field_locators_not_produced_by_document_adapter"
                ),
            )
        except IntakeProducerError:
            raise
        except Exception as exc:
            raise IntakeProducerError(f"解析失败: {exc}") from exc

    async def _produce_payment(
        self, command: IntakeCommand, file_content: str, llm_adapter: LLMAdapter
    ) -> PaymentScheduleIntakeResponse:
        try:
            content = await llm_adapter.complete(
                system=(
                    "你是一位专业的 IFRS 16 租金表解析专家。请准确提取付款计划信息，"
                    "严格遵守日期格式和金额格式要求。"
                ),
                prompt=_payment_prompt(file_content),
                temperature=0.1,
                max_tokens=4000,
            )
            parsed = _parse_payment_llm_content(content)
        except Exception as exc:
            reason = f"{type(exc).__name__}: {str(exc) or repr(exc)}"
            parsed = _fallback_parse_payment_schedule_text(file_content, reason)

        schedules, missing_fields, warnings = _validate_payment_schedules(parsed)
        overall_confidence = _sanitize_confidence(parsed.get("overall_confidence"))
        return build_payment_schedule_intake(
            file_id=command.file_id,
            object_name=command.object_name,
            content_type=command.content_type,
            schedules=schedules,
            confidence_scores={
                "overall": overall_confidence,
                "average_item": sum(item.confidence for item in schedules)
                / max(len(schedules), 1),
            },
            missing_fields=sorted(set(missing_fields)),
            warnings=warnings + ["AI Assist Mode: 付款计划草稿需人工确认后入库"],
            evidence_missing_reason=("field_locators_not_produced_by_document_adapter"),
        )

    async def _produce_contract_batch(
        self,
        command: IntakeCommand,
        material: SourceMaterial,
        llm_adapter: LLMAdapter,
    ) -> ContractBatchIntakeResponse:
        deterministic = False
        fallback_warning: str | None = None
        try:
            content = await llm_adapter.complete(
                system=(
                    "你是一位专业的 IFRS 16 租赁合同台账解析专家。请准确提取每份合同字段。"
                    "如果字段未在合同中出现，不要猜测，标记为缺失。"
                ),
                prompt=_contract_batch_prompt(material.text),
                temperature=0.1,
                max_tokens=8000,
                response_format={"type": "json_object"},
            )
            try:
                parsed = json.loads(content)
            except (TypeError, json.JSONDecodeError):
                parsed = {
                    "contracts": [],
                    "overall_confidence": 0.0,
                    "missing_fields": [],
                    "warnings": ["解析响应格式异常，请人工检查"],
                }
            if not parsed.get("contracts") and material.deterministic_records:
                deterministic = True
                fallback_warning = (
                    "LLM 主解析未能从该 Excel 台账稳定提取合同，已启用表格读取兜底；"
                    "这不是正式入库结果，必须人工逐条确认。"
                )
                parsed = {"contracts": material.deterministic_records}
        except Exception as exc:
            if not material.deterministic_records:
                raise IntakeProducerError(f"批量解析失败: {exc}") from exc
            deterministic = True
            fallback_warning = (
                "LLM 主解析暂不可用或返回异常，已启用 Excel 表格读取兜底；"
                f"合同草稿必须人工逐条确认。原因: {exc}"
            )
            parsed = {"contracts": material.deterministic_records}

        contracts: list[ContractDraftData] = []
        warnings = [str(item) for item in _as_list(parsed.get("warnings"))]
        missing = [str(item) for item in _as_list(parsed.get("missing_fields"))]
        if fallback_warning:
            warnings.insert(0, fallback_warning)
        for index, raw in enumerate(parsed.get("contracts") or []):
            candidate = dict(raw)
            if _is_excel_content_type(command.content_type):
                candidate = _apply_excel_evidence_safety_checks(
                    candidate, material.text
                )
            if (
                not candidate.get("contract_number")
                or not candidate.get("lessee")
                or not candidate.get("lessor")
            ):
                if not deterministic:
                    warnings.append(
                        f"第 {index + 1} 份合同缺少必要字段 (contract_number/lessee/lessor)，已跳过"
                    )
                    continue
                if not candidate.get("lessee"):
                    candidate.setdefault("warnings", []).append(
                        f"第 {index + 1} 行缺少承租方/法人主体"
                    )
                if not candidate.get("lessor"):
                    candidate.setdefault("warnings", []).append(
                        f"第 {index + 1} 行缺少出租方"
                    )
            discount_missing, discount_warnings = _check_discount_rate_missing(
                candidate
            )
            currency_missing, currency_warnings = _check_currency_missing(candidate)
            field_missing, field_warnings = _check_critical_fields(candidate)
            _, scope_warnings = _normalize_lease_scope(candidate)
            confidence = _sanitize_confidence(candidate.get("confidence"))
            if field_missing or discount_missing or currency_missing:
                confidence = min(confidence or 0.9, 0.7)
            if _sanitize_confidence(candidate.get("scope_confidence")) < 0.8:
                confidence = min(confidence or 0.9, 0.7)
            item_missing = sorted(
                set(
                    field_missing
                    + (["discount_rate"] if discount_missing else [])
                    + (["currency"] if currency_missing else [])
                )
            )
            item_warnings = (
                discount_warnings
                + currency_warnings
                + field_warnings
                + scope_warnings
                + [str(item) for item in _as_list(candidate.get("warnings"))]
            )
            normalized = _normalize_contract_record(candidate)
            normalized.update(
                confidence=confidence,
                missing_fields=item_missing,
                warnings=item_warnings,
            )
            if deterministic:
                normalized["scope_source"] = "ledger"
            contracts.append(ContractDraftData.model_validate(normalized))
            warnings.extend(item_warnings)
            missing.extend(item_missing)

        overall = _sanitize_confidence(parsed.get("overall_confidence"))
        if deterministic or overall <= 0:
            overall = sum(item.confidence for item in contracts) / max(
                len(contracts), 1
            )
        warnings.append("AI Assist Mode: 合同台账草稿需人工逐条确认后入库")
        evidence_locators = material.evidence_locators if deterministic else None
        return build_contract_batch_intake(
            file_id=command.file_id,
            object_name=command.object_name,
            content_type=command.content_type,
            contracts=contracts,
            confidence_scores={
                "overall": overall,
                "average_item": sum(item.confidence for item in contracts)
                / max(len(contracts), 1),
            },
            missing_fields=sorted(set(missing)),
            warnings=warnings,
            evidence_locators=evidence_locators,
            evidence_complete=bool(deterministic and evidence_locators),
            evidence_missing_reason=(
                "field_locators_not_produced_by_llm_adapter"
                if _is_excel_content_type(command.content_type)
                else "field_locators_not_produced_by_document_adapter"
            ),
        )


def _contract_prompt(file_content: str) -> str:
    return f"""
    你是一位专业的 IFRS 16 租赁合同解析专家。请从以下合同文本中提取关键字段。

    【重要规则 — 必须遵守】
    1. 如果合同中未明确提到折现率/利率，不要猜测，标记为缺失
    2. 如果合同中未明确提到货币（币种），不要猜测，标记为缺失或 unknown
    3. 区分先付租金(prepaid)和后付租金(postpaid)
    4. 区分固定租金和变量租金
    5. 识别租赁成分和非租赁成分(CAM、服务费)
    6. 承租方(lessee)是租赁合同的乙方，即使用物业并支付租金的一方
    7. 出租方(lessor)是租赁合同的甲方，即提供物业并收取租金的一方
    8. 必须做 IFRS 16 范围初判：是否存在已识别资产、承租方是否控制使用、租期是否 ≤12 个月、是否低价值资产。AI 只能建议，不能直接入正式账。

    合同文本:
    {file_content}

    请提取以下字段（JSON 格式）:
    - contract_number: 合同编号
    - contract_name: 合同名称
    - lessee: 承租方名称（合同中承租人/乙方对应的完整公司名称）
    - lessor: 出租方名称（合同中出租人/甲方对应的完整公司名称）
    - store_name: 门店/物业名称（如有明确提及，否则留空）
    - store_address: 门店/物业地址（如有明确提及，否则留空）
    - commencement_date: 租赁起始日 (YYYY-MM-DD)
    - lease_start_date: 租赁开始日 (YYYY-MM-DD)
    - lease_end_date: 租期结束日 (YYYY-MM-DD)
    - currency: 币种 (CNY/USD/EUR)。如果合同中没有明确提到货币，返回 null 或空字符串，不要猜测
    - asset_type: 标的资产类型 "real_estate" | "vehicle" | "it_equipment" | "machinery" | "other"
    - fixed_rent_amount: 固定租金金额（仅数字，不含货币单位）
    - payment_frequency: 付款频率 (monthly/quarterly/yearly)
    - payment_timing: 付款时点 (prepaid/postpaid)。如果合同写明"每月X日前支付"则为prepaid；"每月X日后支付"或"月末支付"则为postpaid
    - renewal_option: 是否有续租选择权 (true/false)
    - termination_option: 是否有终止选择权 (true/false)
    - cam_amount: 物业管理费 (如有，仅数字)
    - service_fee: 服务费 (如有，仅数字)
    - discount_rate_type: 折现率类型 (如合同中提及)
    - discount_rate: 折现率数值 (如合同中提及)
    - is_lease: 是否构成 IFRS 16 租赁 (true/false)
    - suggested_scope: "in_scope" | "short_term_exempt" | "low_value_exempt" | "not_a_lease"
    - exemption_reason: 范围判定依据，如"租期 10 个月且无续租意图"、"未识别特定资产"
    - scope_confidence: 范围判定置信度 (0-1)

    请以 JSON 格式输出，包含:
    - extracted_fields: 提取的字段
    - confidence_scores: 每个字段的置信度 (0-1)
    - overall_confidence: 总体置信度
    - missing_fields: 识别为缺失的字段列表
    - warnings: 需要人工注意的问题列表
    """


def _payment_prompt(file_content: str) -> str:
    return f"""
你是一位专业的 IFRS 16 租金表解析专家。请从以下租金表内容中提取付款计划信息。

【重要规则 - 必须遵守】
1. 每笔付款必须识别：先付(prepaid)还是后付(postpaid)
   - 先付：在覆盖期间开始前支付（如月初预付当月租金）
   - 后付：在覆盖期间结束后支付（如月末支付当月租金）
2. 区分固定租金和变量租金（turnover rent / sales-based rent 必须标记为变量）
3. 区分租赁成分和非租赁成分（CAM、管理费、服务费等）
4. 金额必须是数字，不要包含货币符号
5. 日期格式必须为 YYYY-MM-DD
6. 如果租金表是月度数据，期间起始日=当月1日，期间结束日=当月最后一日
7. 如果某期金额为空或为0，跳过该行

租金表内容:
{file_content}

请提取每笔付款，以 JSON 数组格式输出。每个元素包含:
- period_start: 覆盖期间起始日 (YYYY-MM-DD)
- period_end: 覆盖期间结束日 (YYYY-MM-DD)
- due_date: 应付日期 (YYYY-MM-DD)
- amount: 金额 (纯数字)
- payment_timing: "prepaid" 或 "postpaid"
- is_fixed: true/false
- is_lease_component: true/false
- amount_type: "fixed_rent" | "turnover_rent" | "cam" | "service_fee" | "tax" | "deposit" | "other"
- currency: "CNY" 或文件中出现的币种。如果文件中未明确提到货币，返回 null 或空字符串，不要猜测
- confidence: 该笔识别的置信度 (0.0-1.0)

额外输出字段（JSON 对象顶层）:
- overall_confidence: 总体置信度 (0.0-1.0)
- missing_fields: 识别中遇到问题的字段列表
- warnings: 需要人工注意的问题列表（如：日期格式不确定、金额可能有误等）
- total_schedules: 识别到的付款笔数

请以纯 JSON 格式输出，不要包含任何 markdown 代码块标记。
"""


def _contract_batch_prompt(file_content: str) -> str:
    return f"""
    你是一位专业的 IFRS 16 租赁合同台账解析专家。请从以下合同台账内容中提取每一份合同的字段。

    【重要规则 — 必须遵守】
    1. 台账中可能包含多份合同，请逐条提取
    2. 如果合同中未明确提到折现率/利率，不要猜测，标记为缺失
    3. 如果合同中未明确提到货币（币种），不要猜测，标记为缺失或 unknown
    4. 区分先付租金(prepaid)和后付租金(postpaid)
    5. 区分固定租金和变量租金
    6. 识别租赁成分和非租赁成分(CAM、服务费)
    7. 承租方(lessee)是租赁合同的乙方，即使用物业并支付租金的一方
    8. 出租方(lessor)是租赁合同的甲方，即提供物业并收取租金的一方
    9. 必须做 IFRS 16 范围初判：是否存在已识别资产、承租方是否控制使用、租期是否 ≤12 个月、是否低价值资产。AI 只能建议，不能直接入正式账。
    10. 如果内容来自 Excel，台账可能是非标准排版、多 sheet、多行标题、合并单元格展开后的文本；请按语义理解 sheet 名、标题行、相邻单元格和字段含义，不要依赖固定列名或固定顺序。
    11. 如果出现"法人主体"、"租赁主体"、"承租公司"等字段，通常可作为 lessee/承租方；但仍需结合上下文判断。
    12. 续租/终止选择权必须按否定语义优先判断："不行使"、"未行使"、"不会行使"、"不合理确定"、"无" 均为 false。不要因为文本出现"终止选择权"或"续租选择权"几个字就返回 true。

    合同台账内容:
    {file_content}

    请以 JSON 格式输出，包含以下顶层字段:
    - contracts: 合同列表，每个元素包含:
      - contract_number: 合同编号
      - contract_name: 合同名称
      - lessee: 承租方名称
      - lessor: 出租方名称
      - store_name: 门店/物业名称（如有）
      - store_address: 门店/物业地址（如有）
      - commencement_date: 租赁起始日 (YYYY-MM-DD)
      - lease_start_date: 租赁开始日 (YYYY-MM-DD)
      - lease_end_date: 租期结束日 (YYYY-MM-DD)
      - currency: 币种 (CNY/USD/EUR)。如果未明确提到，返回 null 或空字符串，不要猜测
      - asset_type: 标的资产类型 "real_estate" | "vehicle" | "it_equipment" | "machinery" | "other"
      - fixed_rent_amount: 固定租金金额（仅数字）
      - payment_frequency: 付款频率 (monthly/quarterly/yearly)
      - payment_timing: 付款时点 (prepaid/postpaid)
      - renewal_option: 是否有续租选择权 (true/false)
      - termination_option: 是否有终止选择权 (true/false)
      - cam_amount: 物业管理费 (如有，仅数字)
      - service_fee: 服务费 (如有，仅数字)
      - discount_rate_type: 折现率类型 (如合同中提及)
      - discount_rate: 折现率数值 (如合同中提及)
      - is_lease: 是否构成 IFRS 16 租赁 (true/false)
      - suggested_scope: "in_scope" | "short_term_exempt" | "low_value_exempt" | "not_a_lease"
      - exemption_reason: 范围判定依据，如"租期 10 个月且无续租意图"、"未识别特定资产"
      - scope_confidence: 范围判定置信度 (0-1)
      - confidence: 该份合同识别的置信度 (0-1)
      - missing_fields: 该份合同缺失的字段列表
      - warnings: 该份合同的警告列表
    - total_count: 识别到的合同总数
    - overall_confidence: 总体置信度 (0-1)
    - missing_fields: 全局缺失字段汇总
    - warnings: 全局警告列表
    """


def _parse_payment_llm_content(content: str) -> dict[str, Any]:
    cleaned = content.strip()
    if cleaned.startswith("```json"):
        cleaned = cleaned[7:]
    elif cleaned.startswith("```"):
        cleaned = cleaned[3:]
    if cleaned.endswith("```"):
        cleaned = cleaned[:-3]
    cleaned = cleaned.strip()
    try:
        parsed = json.loads(cleaned)
        if isinstance(parsed, list):
            return {
                "schedules": parsed,
                "overall_confidence": 0.8,
                "missing_fields": [],
                "warnings": [],
                "total_schedules": len(parsed),
            }
        return parsed
    except json.JSONDecodeError:
        start, end = cleaned.find("["), cleaned.rfind("]") + 1
        if start >= 0 and end > start:
            try:
                schedules = json.loads(cleaned[start:end])
                return {
                    "schedules": schedules,
                    "overall_confidence": 0.7,
                    "missing_fields": [],
                    "warnings": ["JSON 解析使用了启发式提取"],
                    "total_schedules": len(schedules),
                }
            except Exception:
                pass
        return {
            "schedules": [],
            "overall_confidence": 0.0,
            "missing_fields": ["all"],
            "warnings": ["无法解析 LLM 输出"],
            "total_schedules": 0,
        }


def _validate_payment_schedules(
    parsed: dict[str, Any],
) -> tuple[list[PaymentScheduleItem], list[str], list[str]]:
    validated: list[PaymentScheduleItem] = []
    warnings = [str(item) for item in _as_list(parsed.get("warnings"))]
    missing_fields = [str(item) for item in _as_list(parsed.get("missing_fields"))]
    for index, schedule in enumerate(parsed.get("schedules") or []):
        if not schedule.get("due_date") or not schedule.get("amount"):
            warnings.append(f"第 {index + 1} 行缺少必要字段 (due_date/amount)，已跳过")
            continue
        for field in ("period_start", "period_end", "due_date"):
            value = schedule.get(field)
            if value and len(str(value)) != 10:
                warnings.append(
                    f"第 {index + 1} 行 {field} 日期格式可能不正确: {value}"
                )
        try:
            amount = float(schedule["amount"])
        except (TypeError, ValueError):
            warnings.append(
                f"第 {index + 1} 行金额无法解析为数字: {schedule.get('amount')}"
            )
            continue
        if amount <= 0:
            warnings.append(f"第 {index + 1} 行金额 <= 0，已跳过")
            continue
        timing = _normalize_payment_timing(schedule.get("payment_timing"))
        if not timing:
            warnings.append(
                f"第 {index + 1} 行缺少有效付款时点 (prepaid/postpaid)，已跳过"
            )
            missing_fields.append("payment_timing")
            continue
        validated.append(
            PaymentScheduleItem(
                period_start=schedule.get("period_start")
                or schedule.get("due_date", ""),
                period_end=schedule.get("period_end") or schedule.get("due_date", ""),
                due_date=schedule["due_date"],
                amount=amount,
                payment_timing=timing,
                is_fixed=schedule.get("is_fixed", True),
                is_lease_component=schedule.get("is_lease_component", True),
                amount_type=schedule.get("amount_type") or "fixed_rent",
                currency=schedule.get("currency") or "",
                confidence=_sanitize_confidence(schedule.get("confidence")),
            )
        )
    low_confidence = sum(item.confidence < 0.8 for item in validated)
    if low_confidence:
        warnings.append(f"有 {low_confidence} 笔付款的置信度低于 0.8，建议人工复核")
    if len({item.payment_timing for item in validated}) > 1:
        warnings.append("租金表中同时出现先付和后付，请确认是否正确")
    return validated, missing_fields, warnings


def _fallback_parse_payment_schedule_text(
    file_content: str, reason: str
) -> dict[str, Any]:
    rows: list[list[str]] = []
    for raw_line in file_content.splitlines():
        line = raw_line.strip()
        if (
            not line
            or line.startswith("###")
            or set(line.replace("|", "").replace("-", "").strip()) == set()
        ):
            continue
        if "|" in line:
            cells = [cell.strip() for cell in line.split("|")]
            if len(cells) >= 3:
                rows.append(cells)
    if not rows:
        return {
            "schedules": [],
            "overall_confidence": 0.0,
            "missing_fields": ["all"],
            "warnings": [f"LLM 解析不可用，且 Office 表格兜底未找到可读行: {reason}"],
        }
    header = [_normalize_header(cell) for cell in rows[0]]
    required = {"due_date", "amount"}
    if not required.issubset(header):
        return {
            "schedules": [],
            "overall_confidence": 0.0,
            "missing_fields": sorted(required - set(header)),
            "warnings": [f"LLM 解析不可用，Office 表格兜底无法识别必要列: {reason}"],
        }
    schedules: list[dict[str, Any]] = []
    data_rows = (
        rows[2:]
        if len(rows) > 1 and all(not cell.replace("-", "").strip() for cell in rows[1])
        else rows[1:]
    )
    for cells in data_rows:
        row = {
            header[index]: cells[index] if index < len(cells) else ""
            for index in range(len(header))
        }
        if not row.get("due_date") or not row.get("amount"):
            continue
        try:
            amount = float(str(row["amount"]).replace(",", ""))
        except ValueError:
            continue
        amount_type = row.get("amount_type") or "fixed_rent"
        component = row.get("component", "")
        schedules.append(
            {
                "period_start": row.get("period_start") or row["due_date"],
                "period_end": row.get("period_end") or row["due_date"],
                "due_date": row["due_date"],
                "amount": amount,
                "payment_timing": _normalize_payment_timing(row.get("payment_timing")),
                "is_fixed": _truthy_cell(
                    row.get("is_fixed", ""),
                    amount_type not in {"turnover_rent", "variable_rent"},
                ),
                "is_lease_component": _truthy_cell(
                    row.get("is_lease_component", ""),
                    component != "non_lease"
                    and amount_type not in {"cam", "service_fee"},
                ),
                "amount_type": amount_type,
                "currency": row.get("currency") or "",
                "confidence": 0.65,
            }
        )
    return {
        "schedules": schedules,
        "overall_confidence": 0.65 if schedules else 0.0,
        "missing_fields": [] if schedules else ["all"],
        "warnings": [
            f"LLM 解析不可用，已使用 Office 表格兜底读取，必须人工复核: {reason}"
        ],
    }


def _normalize_header(value: str) -> str:
    normalized = value.strip().lower().replace(" ", "_").replace("-", "_")
    aliases = {
        "覆盖期间起始日": "period_start",
        "期间开始": "period_start",
        "覆盖期间结束日": "period_end",
        "期间结束": "period_end",
        "应付日期": "due_date",
        "应付日": "due_date",
        "金额": "amount",
        "付款时点": "payment_timing",
        "金额类型": "amount_type",
        "币种": "currency",
        "成分": "component",
        "固定租金": "is_fixed",
        "租赁成分": "is_lease_component",
    }
    return aliases.get(normalized, normalized)


def _truthy_cell(value: str, default: bool) -> bool:
    normalized = value.strip().lower()
    if normalized in {"true", "yes", "y", "1", "是", "租赁", "lease"}:
        return True
    if normalized in {
        "false",
        "no",
        "n",
        "0",
        "否",
        "非租赁",
        "non_lease",
        "non-lease",
    }:
        return False
    return default


def _check_discount_rate_missing(extracted: dict[str, Any]) -> tuple[bool, list[str]]:
    if extracted.get("discount_rate_type") or extracted.get("discount_rate"):
        return False, []
    return True, [
        "【关键】合同缺少折现率信息。AI 不得猜测折现率，需要人工确认。",
        "建议处理方式：",
        "1. 检查合同文本中是否明确提到利率",
        "2. 从系统政策库中查找适用的 IBR",
        "3. 按法人/租期区间/门店类型匹配利率政策",
        "4. 如无法唯一确定，请人工输入或选择",
    ]


def _check_currency_missing(extracted: dict[str, Any]) -> tuple[bool, list[str]]:
    currency = extracted.get("currency")
    if (
        currency
        and str(currency).strip()
        and str(currency).lower()
        not in {
            "unknown",
            "null",
            "none",
        }
    ):
        return False, []
    return True, [
        "【必须确认】AI 未识别到合同货币。根据 IFRS 16 计量要求，货币直接影响租赁负债现值计算和后续会计分录。",
        "请在上传后手动选择货币（CNY / USD / EUR 等）。AI 不会猜测货币。",
    ]


def _check_critical_fields(extracted: dict[str, Any]) -> tuple[list[str], list[str]]:
    fields = [
        ("contract_number", "合同编号"),
        ("lessee", "承租方"),
        ("lessor", "出租方"),
        ("commencement_date", "租赁起始日"),
        ("lease_start_date", "租赁开始日"),
        ("lease_end_date", "租期结束日"),
        ("fixed_rent_amount", "固定租金金额"),
        ("payment_timing", "付款时点（先付/后付）"),
    ]
    missing = [field for field, _ in fields if not extracted.get(field)]
    labels = {field: label for field, label in fields}
    return missing, [
        f"【关键字段缺失】{labels[field]}({field}) 未识别到，必须人工补充"
        for field in missing
    ]


def _normalize_lease_scope(extracted: dict[str, Any]) -> tuple[str, list[str]]:
    warnings: list[str] = []
    scope = str(
        extracted.get("suggested_scope") or extracted.get("lease_scope") or "in_scope"
    ).strip()
    allowed = {"in_scope", "short_term_exempt", "low_value_exempt", "not_a_lease"}
    if scope not in allowed:
        warnings.append(
            "【范围判定】AI 未能给出有效 lease_scope，默认按 in_scope 进入人工确认。"
        )
        scope = "in_scope"
    confidence = _optional_float(extracted.get("scope_confidence"))
    if confidence is not None:
        confidence = _sanitize_confidence(confidence)
    if confidence is None or confidence < 0.8:
        warnings.append(
            "【必须确认】租赁范围判定置信度不足，需要人工确认是否资本化、短期/低价值豁免或非租赁。"
        )
    extracted.update(
        lease_scope=scope, suggested_scope=scope, scope_source="ai_suggested"
    )
    if confidence is not None:
        extracted["scope_confidence"] = confidence
    return scope, warnings


def _sanitize_confidence_scores(scores: dict[str, Any]) -> dict[str, float]:
    return {key: _sanitize_confidence(value) for key, value in scores.items()}


def _sanitize_confidence(value: Any) -> float:
    return min(max(_parse_float(value), 0.0), 1.0)


def _normalize_payment_timing(value: Any) -> str:
    text = str(value or "").strip().lower()
    if text in {"prepaid", "advance", "先付", "期初", "预付"}:
        return "prepaid"
    if text in {"postpaid", "arrears", "后付", "期末"}:
        return "postpaid"
    return ""


def _normalize_asset_type(value: Any) -> str:
    text = str(value or "").strip().lower()
    if any(keyword in text for keyword in ("店", "铺", "物业", "房", "real")):
        return "real_estate"
    if any(keyword in text for keyword in ("车", "vehicle")):
        return "vehicle"
    if any(keyword in text for keyword in ("电脑", "it", "服务器", "设备")):
        return "it_equipment"
    if any(keyword in text for keyword in ("机器", "machinery")):
        return "machinery"
    return "other" if text else "real_estate"


def _normalize_payment_frequency(value: Any, default: str = "monthly") -> str:
    frequency = str(value or default).strip().lower()
    if frequency in {"monthly", "quarterly", "yearly"}:
        return frequency
    return ""


def _normalize_contract_record(
    candidate: dict[str, Any], default_payment_frequency: str = "monthly"
) -> dict[str, Any]:
    scope = (
        candidate.get("lease_scope") or candidate.get("suggested_scope") or "in_scope"
    )
    return {
        "contract_number": candidate.get("contract_number") or "",
        "contract_name": candidate.get("contract_name") or "",
        "lessee": candidate.get("lessee") or "",
        "lessor": candidate.get("lessor") or "",
        "store_name": candidate.get("store_name") or "",
        "store_address": candidate.get("store_address") or "",
        "commencement_date": candidate.get("commencement_date") or "",
        "lease_start_date": candidate.get("lease_start_date") or "",
        "lease_end_date": candidate.get("lease_end_date") or "",
        "currency": candidate.get("currency") or "",
        "asset_type": _normalize_asset_type(candidate.get("asset_type")),
        "fixed_rent_amount": _parse_float(candidate.get("fixed_rent_amount")),
        "payment_frequency": _normalize_payment_frequency(
            candidate.get("payment_frequency"), default_payment_frequency
        ),
        "payment_timing": _normalize_payment_timing(candidate.get("payment_timing")),
        "renewal_option": _coerce_bool(candidate.get("renewal_option")),
        "termination_option": _coerce_bool(candidate.get("termination_option")),
        "cam_amount": _parse_float(candidate.get("cam_amount")),
        "service_fee": _parse_float(candidate.get("service_fee")),
        "discount_rate_type": candidate.get("discount_rate_type") or "",
        "discount_rate": _parse_float(candidate.get("discount_rate")),
        "is_lease": _coerce_bool(candidate.get("is_lease"), scope != "not_a_lease"),
        "lease_scope": scope,
        "suggested_scope": candidate.get("suggested_scope") or scope,
        "exemption_reason": candidate.get("exemption_reason") or "",
        "scope_source": candidate.get("scope_source") or "ai_suggested",
        "scope_confidence": _sanitize_confidence(candidate.get("scope_confidence")),
    }


def _parse_float(value: Any) -> float:
    if value is None:
        return 0.0
    if isinstance(value, (int, float)):
        return float(value)
    text = (
        str(value)
        .strip()
        .replace(",", "")
        .replace("%", "")
        .replace("¥", "")
        .replace("￥", "")
        .replace("元", "")
    )
    if not text or any(
        marker in text.lower() for marker in ("缺失", "待确认", "unknown", "null")
    ):
        return 0.0
    try:
        return float(text)
    except ValueError:
        return 0.0


def _optional_float(value: Any) -> float | None:
    if value is None or value == "":
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def _coerce_bool(value: Any, default: bool = False) -> bool:
    if value is None or value == "":
        return default
    if isinstance(value, bool):
        return value
    if isinstance(value, (int, float)):
        return value != 0
    text = str(value).strip().lower()
    if text in {"true", "yes", "y", "1", "是", "有"}:
        return True
    if text in {"false", "no", "n", "0", "否", "无"}:
        return False
    if any(marker in text for marker in ("不行使", "未行使", "不会行使", "不合理确定")):
        return False
    if any(marker in text for marker in ("合理确定", "行使")):
        return True
    return default


def _apply_excel_evidence_safety_checks(
    contract: dict[str, Any], file_content: str
) -> dict[str, Any]:
    contract_number = str(contract.get("contract_number") or "").strip()
    if not contract_number or not file_content:
        return contract
    evidence_line = next(
        (line for line in file_content.splitlines() if contract_number in line), ""
    )
    if "终止" in evidence_line and any(
        keyword in evidence_line
        for keyword in ("不行使", "未行使", "不会行使", "不合理确定")
    ):
        contract["termination_option"] = False
    if "续租" in evidence_line and any(
        keyword in evidence_line
        for keyword in (
            "不续租",
            "不行使续租",
            "未行使续租",
            "不会续租",
            "不合理确定续租",
        )
    ):
        contract["renewal_option"] = False
    return contract


def _is_excel_content_type(content_type: str) -> bool:
    return content_type in {
        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        "application/vnd.ms-excel",
    }


def _as_list(value: Any) -> list[Any]:
    if value is None:
        return []
    return value if isinstance(value, list) else [value]
