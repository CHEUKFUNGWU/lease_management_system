"""Typed, versioned seam between AI adapters and the core service.

Adapters may change independently, but responses crossing this seam must keep
their evidence, confidence, and review policy explicit.
"""

from typing import Literal, Optional
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
    confidence: float = Field(default=0.9, ge=0, le=1)


class PaymentScheduleIntakeResponse(BaseModel):
    schema_version: Literal["ai-intake.v1"] = SCHEMA_VERSION
    intake_id: str
    task_id: str
    file_id: str
    mode: Literal["assist"] = "assist"
    draft_type: Literal["payment_schedule_draft"] = "payment_schedule_draft"
    status: Literal["draft_generated"] = "draft_generated"
    schedules: list[PaymentScheduleItem]
    confidence_scores: dict[str, float]
    missing_fields: list[str]
    warnings: list[str]
    requires_human_confirmation: Literal[True] = True
    evidence: EvidenceBundle
    review_gate: ReviewGate

    @model_validator(mode="after")
    def validate_source_identity(self) -> "PaymentScheduleIntakeResponse":
        if self.evidence.source_file_id != self.file_id:
            raise ValueError("evidence source must match the intake file")
        return self


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
    evidence_missing_reason: Optional[str] = None,
) -> PaymentScheduleIntakeResponse:
    """Apply the Assist Mode evidence and review policy once."""

    locators = evidence_locators or []
    overall_confidence = confidence_scores.get("overall", 0.0)
    reasons = ["assist_mode"]
    if overall_confidence < ASSIST_REVIEW_THRESHOLD:
        reasons.append("low_confidence")
    if missing_fields:
        reasons.append("missing_fields")
    if warnings:
        reasons.append("warnings_present")
    if not locators:
        reasons.append("evidence_incomplete")

    return PaymentScheduleIntakeResponse(
        intake_id="intake_" + uuid4().hex,
        task_id="task_ps_" + file_id,
        file_id=file_id,
        schedules=schedules,
        confidence_scores=confidence_scores,
        missing_fields=missing_fields,
        warnings=warnings,
        evidence=EvidenceBundle(
            source_file_id=file_id,
            object_name=object_name,
            content_type=content_type,
            locators=locators,
            complete=bool(locators),
            missing_reason=(
                None
                if locators
                else evidence_missing_reason
                or "field_locators_not_produced_by_adapter"
            ),
        ),
        review_gate=ReviewGate(reasons=reasons),
    )
