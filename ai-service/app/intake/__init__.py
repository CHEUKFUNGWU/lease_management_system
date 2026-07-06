"""Versioned AI intake contracts exposed to the core service."""

from app.intake.models import (
    ContractBatchIntakeResponse,
    ContractDraftData,
    ContractIntakeResponse,
    PaymentScheduleIntakeResponse,
    PaymentScheduleItem,
    build_contract_batch_intake,
    build_contract_intake,
    build_payment_schedule_intake,
)
from app.intake.producer import (
    AIIntakeProducer,
    IntakeCommand,
    IntakeKind,
    IntakeProducerError,
)

__all__ = [
    "ContractBatchIntakeResponse",
    "ContractDraftData",
    "ContractIntakeResponse",
    "PaymentScheduleIntakeResponse",
    "PaymentScheduleItem",
    "build_contract_batch_intake",
    "build_contract_intake",
    "build_payment_schedule_intake",
    "AIIntakeProducer",
    "IntakeCommand",
    "IntakeKind",
    "IntakeProducerError",
]
