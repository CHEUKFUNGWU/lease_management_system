from typing import List, Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

from app.intake.adapters import (
    ProvidedTextAdapter,
    RuntimeLLMAdapter,
    StoredDocumentAdapter,
    StoredExcelAdapter,
)
from app.intake.models import (
    ContractBatchIntakeResponse,
    ContractIntakeResponse,
    EventIntakeResponse,
    PaymentScheduleIntakeResponse,
)
from app.intake.producer import (
    AIIntakeProducer,
    IntakeCommand,
    IntakeKind,
    IntakeProducerError,
)
from app.services.document_extractor import extract_text, extract_text_with_evidence
from app.services.llm import llm_client
from app.services.storage import download_from_minio


router = APIRouter()
intake_producer = AIIntakeProducer()


class ParseRequest(BaseModel):
    file_id: str
    file_type: str
    task_type: Optional[str] = "contract_extraction"


class ParseResponse(BaseModel):
    task_id: str
    file_id: str
    status: str
    extracted_data: Optional[dict] = None
    confidence_scores: Optional[dict] = None
    warnings: Optional[List[str]] = None


class ContractDraftRequest(BaseModel):
    file_id: str
    object_name: str
    content_type: str = "application/pdf"
    file_content: Optional[str] = None
    mode: str = "assist"


class PaymentScheduleParseRequest(BaseModel):
    file_id: str
    object_name: str
    content_type: str
    mode: str = "assist"


class EventParseRequest(BaseModel):
    file_id: str
    object_name: str
    content_type: str = "application/pdf"
    contract_id: str = ""
    mode: str = "assist"


class ContractBatchDraftRequest(BaseModel):
    file_id: str
    object_name: str
    content_type: str = "application/pdf"
    file_content: Optional[str] = None
    mode: str = "assist"


@router.post("/parse", response_model=ParseResponse)
async def parse_document(request: ParseRequest):
    return {
        "task_id": "task_" + request.file_id,
        "file_id": request.file_id,
        "status": "pending",
        "extracted_data": None,
        "confidence_scores": None,
        "warnings": ["AI Assist Mode: 识别结果需人工确认后入库"],
    }


@router.post("/parse/contract", response_model=ContractIntakeResponse)
async def parse_contract(request: ContractDraftRequest):
    return await _produce(
        IntakeKind.CONTRACT,
        request.file_id,
        request.object_name,
        request.content_type,
        request.mode,
        request.file_content,
    )


@router.post("/parse/payment-schedule", response_model=PaymentScheduleIntakeResponse)
async def parse_payment_schedule(request: PaymentScheduleParseRequest):
    return await _produce(
        IntakeKind.PAYMENT_SCHEDULE,
        request.file_id,
        request.object_name,
        request.content_type,
        request.mode,
        None,
    )


@router.post("/parse/event", response_model=EventIntakeResponse)
async def parse_event(request: EventParseRequest):
    return await _produce(
        IntakeKind.EVENT,
        request.file_id,
        request.object_name,
        request.content_type,
        request.mode,
        None,
        request.contract_id,
    )


@router.post("/parse/contract-batch", response_model=ContractBatchIntakeResponse)
async def parse_contract_batch(request: ContractBatchDraftRequest):
    return await _produce(
        IntakeKind.CONTRACT_BATCH,
        request.file_id,
        request.object_name,
        request.content_type,
        request.mode,
        request.file_content,
    )


async def _produce(
    kind: IntakeKind,
    file_id: str,
    object_name: str,
    content_type: str,
    mode: str,
    file_content: Optional[str],
    contract_id: str = "",
):
    command = IntakeCommand(
        kind=kind,
        file_id=file_id,
        object_name=object_name,
        content_type=content_type,
        contract_id=contract_id,
        mode=mode,
    )
    if file_content:
        source_adapter = ProvidedTextAdapter(file_content, content_type)
    elif kind == IntakeKind.CONTRACT_BATCH and _is_excel(content_type):
        source_adapter = StoredExcelAdapter(download_from_minio)
    else:
        source_adapter = StoredDocumentAdapter(
            download_from_minio,
            extract_text,
            extract_with_evidence=extract_text_with_evidence,
        )
    try:
        return await intake_producer.produce(
            command,
            source_adapter,
            RuntimeLLMAdapter(llm_client),
        )
    except IntakeProducerError as exc:
        raise HTTPException(status_code=exc.status_code, detail=exc.detail) from exc


def _is_excel(content_type: str) -> bool:
    return content_type in {
        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        "application/vnd.ms-excel",
    }
