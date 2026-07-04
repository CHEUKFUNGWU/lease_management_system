"""Typed, versioned seam between AI adapters and the core service.

Adapters may change independently, but responses crossing this seam must keep
their evidence, confidence, and review policy explicit.
"""

from typing import Any, Literal, Optional
from uuid import uuid4

from pydantic import BaseModel, Field, model_validator


SCHEMA_VERSION = "ai-intake.v1"
ASSIST_REVIEW_THRESHOLD = 0.8


class EvidenceLocator(BaseModel):
    field: str
    source: str
    page: Optional[int] = None
    coordinates: Optional[list[float]] = None
    quote: str


class EvidenceBundle(BaseModel):
    source_file_id: str
    object_name: str
    content_type: str
    locators: list[EvidenceLocator] = Field(default_factory=list)
    complete: bool
    missing_reason: Optional[str] = None

    @model_validator(mode="after")
    def validate_completeness(self) -> "EvidenceBundle":
        if self.complete and not self.locators:
            raise ValueError("complete evidence requires at least one locator")
        if not self.complete and not self.missing_reason:
            raise ValueError("incomplete evidence requires a missing reason")
        return self


class ReviewGate(BaseModel):
    required: Literal[True] = True
    reasons: list[str]
    confidence_threshold: float = Field(
        default=ASSIST_REVIEW_THRESHOLD, gt=0, le=1
    )


class PaymentScheduleItem(BaseModel):
    period_start: str
    period_end: str
    due_date: str
    amount: float = Field(gt=0)
    payment_timing: Literal["prepaid", "postpaid"]
    is_fixed: bool
    is_lease_component: bool
    amount_type: str = "fixed_rent"
    currency: str = ""
    confidence: float = Field(default=0, ge=0, le=1)


class IntakeResponse(BaseModel):
    schema_version: Literal["ai-intake.v1"] = SCHEMA_VERSION
    intake_id: str
    task_id: str
    file_id: str
    mode: Literal["assist"] = "assist"
    status: Literal["draft_generated"] = "draft_generated"
    confidence_scores: dict[str, float]
    missing_fields: list[str]
    warnings: list[str]
    requires_human_confirmation: Literal[True] = True
    evidence: EvidenceBundle
    review_gate: ReviewGate

    @model_validator(mode="after")
    def validate_source_identity(self) -> "IntakeResponse":
        if self.evidence.source_file_id != self.file_id:
            raise ValueError("evidence source must match the intake file")
        if "overall" not in self.confidence_scores:
            raise ValueError("confidence scores must include overall")
        if any(score < 0 or score > 1 for score in self.confidence_scores.values()):
            raise ValueError("confidence scores must be between zero and one")
        return self


class PaymentScheduleIntakeResponse(IntakeResponse):
    draft_type: Literal["payment_schedule_draft"] = "payment_schedule_draft"
    schedules: list[PaymentScheduleItem]

    @model_validator(mode="after")
    def validate_schedule_evidence(self) -> "PaymentScheduleIntakeResponse":
        if self.evidence.complete:
            _require_record_evidence(self.evidence, "schedules", len(self.schedules))
        return self


class ContractDraftData(BaseModel):
    contract_number: str = ""
    contract_name: str = ""
    lessee: str = ""
    lessor: str = ""
    store_name: str = ""
    store_address: str = ""
    commencement_date: str = ""
    lease_start_date: str = ""
    lease_end_date: str = ""
    currency: str = ""
    asset_type: Literal["real_estate", "vehicle", "it_equipment", "machinery", "other"] = "real_estate"
    fixed_rent_amount: float = Field(default=0, ge=0)
    payment_frequency: Literal["", "monthly", "quarterly", "yearly"] = ""
    payment_timing: Literal["", "prepaid", "postpaid"] = ""
    renewal_option: bool = False
    termination_option: bool = False
    cam_amount: float = Field(default=0, ge=0)
    service_fee: float = Field(default=0, ge=0)
    discount_rate_type: str = ""
    discount_rate: float = Field(default=0, ge=0)
    is_lease: bool = True
    lease_scope: Literal["in_scope", "short_term_exempt", "low_value_exempt", "not_a_lease"] = "in_scope"
    suggested_scope: Literal["in_scope", "short_term_exempt", "low_value_exempt", "not_a_lease"] = "in_scope"
    exemption_reason: str = ""
    scope_source: str = "ai_suggested"
    scope_confidence: float = Field(default=0, ge=0, le=1)
    confidence: float = Field(default=0, ge=0, le=1)
    missing_fields: list[str] = Field(default_factory=list)
    warnings: list[str] = Field(default_factory=list)


class ContractIntakeResponse(IntakeResponse):
    draft_type: Literal["contract_draft"] = "contract_draft"
    extracted_data: ContractDraftData

    @model_validator(mode="after")
    def validate_contract_evidence(self) -> "ContractIntakeResponse":
        if self.evidence.complete:
            _require_record_evidence(self.evidence, "extracted_data", 1)
        return self


class ContractBatchIntakeResponse(IntakeResponse):
    draft_type: Literal["contract_batch_draft"] = "contract_batch_draft"
    contracts: list[ContractDraftData]
    total_count: int = Field(ge=0)

    @model_validator(mode="after")
    def validate_batch(self) -> "ContractBatchIntakeResponse":
        if self.total_count != len(self.contracts):
            raise ValueError("total_count must match the number of contracts")
        if self.evidence.complete:
            _require_record_evidence(self.evidence, "contracts", len(self.contracts))
        return self


def _require_record_evidence(
    evidence: EvidenceBundle, collection: str, count: int
) -> None:
    for index in range(count):
        record_field = (
            collection
            if collection == "extracted_data" and count == 1
            else f"{collection}[{index}]"
        )
        if not any(
            locator.field == record_field
            or locator.field.startswith(record_field + ".")
            for locator in evidence.locators
        ):
            raise ValueError(f"complete evidence does not cover {record_field}")


def _build_intake_metadata(
    *,
    task_prefix: str,
    file_id: str,
    object_name: str,
    content_type: str,
    confidence_scores: dict[str, float],
    missing_fields: list[str],
    warnings: list[str],
    evidence_locators: Optional[list[EvidenceLocator]],
    evidence_complete: bool,
    evidence_missing_reason: Optional[str],
) -> dict[str, Any]:
    locators = evidence_locators or []
    if evidence_complete and not locators:
        raise ValueError("complete evidence requires adapter-provided locators")

    overall_confidence = confidence_scores.get("overall", 0.0)
    reasons = ["assist_mode"]
    if overall_confidence < ASSIST_REVIEW_THRESHOLD:
        reasons.append("low_confidence")
    if missing_fields:
        reasons.append("missing_fields")
    if warnings:
        reasons.append("warnings_present")
    if not evidence_complete:
        reasons.append("evidence_incomplete")

    return {
        "intake_id": "intake_" + uuid4().hex,
        "task_id": task_prefix + file_id,
        "file_id": file_id,
        "confidence_scores": confidence_scores,
        "missing_fields": missing_fields,
        "warnings": warnings,
        "evidence": EvidenceBundle(
            source_file_id=file_id,
            object_name=object_name,
            content_type=content_type,
            locators=locators,
            complete=evidence_complete,
            missing_reason=(
                None
                if evidence_complete
                else evidence_missing_reason
                or "field_locators_not_produced_by_adapter"
            ),
        ),
        "review_gate": ReviewGate(reasons=reasons),
    }


def build_payment_schedule_intake(
    *,
    file_id: str,
    object_name: str,
    content_type: str,
    schedules: list[PaymentScheduleItem],
    confidence_scores: dict[str, float],
    missing_fields: list[str],
    warnings: list[str],
    evidence_locators: Optional[list[EvidenceLocator]] = None,
    evidence_complete: bool = False,
    evidence_missing_reason: Optional[str] = None,
) -> PaymentScheduleIntakeResponse:
    """Apply the Assist Mode evidence and review policy once."""

    return PaymentScheduleIntakeResponse(
        schedules=schedules,
        **_build_intake_metadata(
            task_prefix="task_ps_",
            file_id=file_id,
            object_name=object_name,
            content_type=content_type,
            confidence_scores=confidence_scores,
            missing_fields=missing_fields,
            warnings=warnings,
            evidence_locators=evidence_locators,
            evidence_complete=evidence_complete,
            evidence_missing_reason=evidence_missing_reason,
        ),
    )


def build_contract_intake(
    *,
    file_id: str,
    object_name: str,
    content_type: str,
    extracted_data: dict[str, Any],
    confidence_scores: dict[str, float],
    missing_fields: list[str],
    warnings: list[str],
    evidence_locators: Optional[list[EvidenceLocator]] = None,
    evidence_complete: bool = False,
    evidence_missing_reason: Optional[str] = None,
) -> ContractIntakeResponse:
    """Produce a typed single-contract draft behind the AI intake seam."""

    normalized = _normalize_contract_data(extracted_data)
    if extracted_data.get("confidence") in (None, ""):
        normalized["confidence"] = confidence_scores.get("overall", 0.0)
    normalized["missing_fields"] = missing_fields
    normalized["warnings"] = warnings
    return ContractIntakeResponse(
        extracted_data=ContractDraftData.model_validate(normalized),
        **_build_intake_metadata(
            task_prefix="task_",
            file_id=file_id,
            object_name=object_name,
            content_type=content_type,
            confidence_scores=confidence_scores,
            missing_fields=missing_fields,
            warnings=warnings,
            evidence_locators=evidence_locators,
            evidence_complete=evidence_complete,
            evidence_missing_reason=evidence_missing_reason,
        ),
    )


def build_contract_batch_intake(
    *,
    file_id: str,
    object_name: str,
    content_type: str,
    contracts: list[ContractDraftData | dict[str, Any]],
    confidence_scores: dict[str, float],
    missing_fields: list[str],
    warnings: list[str],
    evidence_locators: Optional[list[EvidenceLocator]] = None,
    evidence_complete: bool = False,
    evidence_missing_reason: Optional[str] = None,
) -> ContractBatchIntakeResponse:
    """Produce typed contract rows from any document/LLM/Excel adapter."""

    typed_contracts = [
        contract
        if isinstance(contract, ContractDraftData)
        else ContractDraftData.model_validate(_normalize_contract_data(contract))
        for contract in contracts
    ]
    return ContractBatchIntakeResponse(
        contracts=typed_contracts,
        total_count=len(typed_contracts),
        **_build_intake_metadata(
            task_prefix="task_batch_",
            file_id=file_id,
            object_name=object_name,
            content_type=content_type,
            confidence_scores=confidence_scores,
            missing_fields=missing_fields,
            warnings=warnings,
            evidence_locators=evidence_locators,
            evidence_complete=evidence_complete,
            evidence_missing_reason=evidence_missing_reason,
        ),
    )


def _normalize_contract_data(extracted_data: dict[str, Any]) -> dict[str, Any]:
    defaults = ContractDraftData().model_dump()
    return {
        key: defaults.get(key) if value is None else value
        for key, value in extracted_data.items()
    }
