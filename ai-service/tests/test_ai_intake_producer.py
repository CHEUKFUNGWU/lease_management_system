import json
import unittest

from app.intake.producer import (
    AIIntakeProducer,
    IntakeCommand,
    IntakeKind,
    IntakeProducerError,
    _resolve_llm_evidence,
)
from app.intake.adapters import SourceMaterial
from app.intake.models import EvidenceLocator


class MemorySourceAdapter:
    async def read(self, command: IntakeCommand, max_characters: int) -> SourceMaterial:
        return SourceMaterial(
            text="contract evidence", content_type=command.content_type
        )


class MemoryLLMAdapter:
    async def complete(self, **_: object) -> str:
        return json.dumps(
            {
                "extracted_fields": {
                    "contract_number": "LEASE-PRODUCER-001",
                    "contract_name": "Producer contract",
                    "lessee": "Lessee",
                    "lessor": "Lessor",
                    "commencement_date": "2026-01-01",
                    "lease_start_date": "2026-01-01",
                    "lease_end_date": "2028-12-31",
                    "currency": "CNY",
                    "asset_type": "real_estate",
                    "fixed_rent_amount": 1000,
                    "payment_frequency": "monthly",
                    "payment_timing": "prepaid",
                    "suggested_scope": "in_scope",
                    "scope_confidence": 0.9,
                },
                "confidence_scores": {"contract_number": 0.95},
                "overall_confidence": 0.9,
                "missing_fields": [],
                "warnings": [],
            }
        )


class AIIntakeProducerTest(unittest.IsolatedAsyncioTestCase):
    def test_llm_evidence_is_resolved_only_against_adapter_locators(self):
        available = [
            EvidenceLocator(
                field="document.pages[0].blocks",
                source="paddleocr:page:1:page[1].parsing_res_list[0]",
                page=1,
                coordinates=[1, 2, 30, 40],
                quote="合同编号：LEASE-001",
            )
        ]
        resolved = _resolve_llm_evidence(
            [
                {"field": "extracted_data.contract_number", "page": 1, "quote": "LEASE-001"},
                {"field": "extracted_data.lessor", "page": 9, "quote": "伪造出租方"},
            ],
            available,
        )
        self.assertEqual(len(resolved), 1)
        self.assertEqual(resolved[0].field, "extracted_data.contract_number")
        self.assertEqual(resolved[0].source, available[0].source)
        self.assertEqual(resolved[0].coordinates, available[0].coordinates)

    async def test_event_production_keeps_complex_event_as_reviewable_draft(self):
        class EventLLMAdapter:
            async def complete(self, **_: object) -> str:
                return json.dumps(
                    {
                        "event": {
                            "event_type": "modification",
                            "effective_date": "",
                            "change_reason": "补充协议调整月租金",
                            "judgment_basis": "补充协议文本",
                            "revision_parameters": {},
                            "field_confidence": {"event_type": 0.72},
                        },
                        "overall_confidence": 0.62,
                        "missing_fields": ["effective_date", "revision_parameters"],
                        "warnings": ["需要核对原文"],
                    }
                )

        result = await AIIntakeProducer().produce(
            IntakeCommand(
                kind=IntakeKind.EVENT,
                file_id="file-event",
                object_name="event.pdf",
                content_type="application/pdf",
                contract_id="contract-1",
            ),
            MemorySourceAdapter(),
            EventLLMAdapter(),
        )

        self.assertEqual(result.draft_type, "event_draft")
        self.assertEqual(result.event.contract_id, "contract-1")
        self.assertIn("effective_date", result.missing_fields)
        self.assertFalse(result.evidence.complete)
        self.assertTrue(result.requires_human_confirmation)

    async def test_producer_rejects_auto_post_before_calling_adapters(self):
        with self.assertRaises(IntakeProducerError) as raised:
            await AIIntakeProducer().produce(
                IntakeCommand(
                    kind=IntakeKind.CONTRACT,
                    file_id="file-auto-post",
                    object_name="contract.pdf",
                    content_type="application/pdf",
                    mode="auto-post",
                ),
                MemorySourceAdapter(),
                MemoryLLMAdapter(),
            )

        self.assertEqual(raised.exception.status_code, 400)

    async def test_contract_production_enforces_ai_intake_policy(self):
        result = await AIIntakeProducer().produce(
            IntakeCommand(
                kind=IntakeKind.CONTRACT,
                file_id="file-producer-contract",
                object_name="contract.pdf",
                content_type="application/pdf",
            ),
            MemorySourceAdapter(),
            MemoryLLMAdapter(),
        )

        self.assertEqual(result.schema_version, "ai-intake.v1")
        self.assertEqual(result.extracted_data.contract_number, "LEASE-PRODUCER-001")
        self.assertIn("discount_rate", result.missing_fields)
        self.assertFalse(result.evidence.complete)
        self.assertEqual(
            result.evidence.missing_reason,
            "field_locators_not_produced_by_document_adapter",
        )
        self.assertTrue(result.review_gate.required)

    async def test_producer_sanitizes_untrusted_confidence_scores(self):
        class UnsafeConfidenceLLMAdapter(MemoryLLMAdapter):
            async def complete(self, **options: object) -> str:
                parsed = json.loads(await super().complete(**options))
                parsed["confidence_scores"] = {
                    "contract_number": 1.7,
                    "lease_end_date": -0.2,
                    "unparseable": "high",
                }
                parsed["overall_confidence"] = 2
                return json.dumps(parsed)

        result = await AIIntakeProducer().produce(
            IntakeCommand(
                kind=IntakeKind.CONTRACT,
                file_id="file-confidence",
                object_name="contract.pdf",
                content_type="application/pdf",
            ),
            MemorySourceAdapter(),
            UnsafeConfidenceLLMAdapter(),
        )

        self.assertEqual(result.confidence_scores["overall"], 1.0)
        self.assertEqual(result.confidence_scores["contract_number"], 1.0)
        self.assertEqual(result.confidence_scores["lease_end_date"], 0.0)
        self.assertEqual(result.confidence_scores["unparseable"], 0.0)

    async def test_payment_production_never_guesses_missing_timing(self):
        class PaymentLLMAdapter:
            async def complete(self, **_: object) -> str:
                return json.dumps(
                    {
                        "schedules": [
                            {
                                "period_start": "2026-01-01",
                                "period_end": "2026-01-31",
                                "due_date": "2026-01-15",
                                "amount": 1000,
                                "is_fixed": True,
                                "is_lease_component": True,
                                "confidence": 0.9,
                            }
                        ],
                        "overall_confidence": 0.9,
                        "missing_fields": [],
                        "warnings": [],
                    }
                )

        result = await AIIntakeProducer().produce(
            IntakeCommand(
                kind=IntakeKind.PAYMENT_SCHEDULE,
                file_id="file-producer-payment",
                object_name="schedule.pdf",
                content_type="application/pdf",
            ),
            MemorySourceAdapter(),
            PaymentLLMAdapter(),
        )

        self.assertEqual(result.schedules, [])
        self.assertIn("payment_timing", result.missing_fields)
        self.assertFalse(result.evidence.complete)
        self.assertTrue(result.review_gate.required)

    async def test_payment_llm_failure_uses_reviewable_table_fallback(self):
        class TableSourceAdapter:
            async def read(
                self, command: IntakeCommand, max_characters: int
            ) -> SourceMaterial:
                return SourceMaterial(
                    text=(
                        "due_date | amount | payment_timing | currency\n"
                        "2026-01-31 | 1000 | 后付 | CNY"
                    ),
                    content_type=command.content_type,
                )

        class FailingLLMAdapter:
            async def complete(self, **_: object) -> str:
                raise RuntimeError("provider unavailable")

        result = await AIIntakeProducer().produce(
            IntakeCommand(
                kind=IntakeKind.PAYMENT_SCHEDULE,
                file_id="file-payment-fallback",
                object_name="schedule.xlsx",
                content_type="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
            ),
            TableSourceAdapter(),
            FailingLLMAdapter(),
        )

        self.assertEqual(len(result.schedules), 1)
        self.assertEqual(result.schedules[0].payment_timing, "postpaid")
        self.assertEqual(result.schedules[0].confidence, 0.65)
        self.assertTrue(
            any("Office 表格兜底" in warning for warning in result.warnings)
        )
        self.assertTrue(result.review_gate.required)

    async def test_excel_batch_fallback_preserves_truthful_row_evidence(self):
        class ExcelSourceAdapter:
            async def read(
                self, command: IntakeCommand, max_characters: int
            ) -> SourceMaterial:
                return SourceMaterial(
                    text="Row 2: A2=LEASE-EXCEL-001",
                    content_type=command.content_type,
                    deterministic_records=[
                        {
                            "contract_number": "LEASE-EXCEL-001",
                            "contract_name": "Excel contract",
                            "lessee": "Lessee",
                            "lessor": "Lessor",
                            "commencement_date": "2026-03-01",
                            "lease_start_date": "2026-03-01",
                            "lease_end_date": "2029-02-28",
                            "currency": "CNY",
                            "fixed_rent_amount": 3000,
                            "payment_timing": "postpaid",
                            "discount_rate": 0.05,
                            "lease_scope": "in_scope",
                            "scope_confidence": 0.9,
                            "confidence": 0.9,
                        }
                    ],
                    evidence_locators=[
                        EvidenceLocator(
                            field="contracts[0]",
                            source="Leases!A2:L2",
                            quote="LEASE-EXCEL-001",
                        )
                    ],
                )

        class EmptyBatchLLMAdapter:
            async def complete(self, **_: object) -> str:
                return json.dumps(
                    {
                        "contracts": [],
                        "overall_confidence": 0,
                        "missing_fields": [],
                        "warnings": [],
                    }
                )

        result = await AIIntakeProducer().produce(
            IntakeCommand(
                kind=IntakeKind.CONTRACT_BATCH,
                file_id="file-producer-excel",
                object_name="leases.xlsx",
                content_type="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
            ),
            ExcelSourceAdapter(),
            EmptyBatchLLMAdapter(),
        )

        self.assertEqual(result.total_count, 1)
        self.assertEqual(result.contracts[0].contract_number, "LEASE-EXCEL-001")
        self.assertTrue(result.evidence.complete)
        self.assertEqual(result.evidence.locators[0].source, "Leases!A2:L2")
        self.assertTrue(result.review_gate.required)

    async def test_llm_batch_never_claims_unrelated_excel_row_evidence(self):
        class ExcelSourceAdapter:
            async def read(
                self, command: IntakeCommand, max_characters: int
            ) -> SourceMaterial:
                return SourceMaterial(
                    text="Row 2: A2=SOURCE-ROW",
                    content_type=command.content_type,
                    deterministic_records=[
                        {
                            "contract_number": "SOURCE-ROW",
                            "lessee": "Source lessee",
                            "lessor": "Source lessor",
                        }
                    ],
                    evidence_locators=[
                        EvidenceLocator(
                            field="contracts[0]",
                            source="Leases!A2:C2",
                            quote="SOURCE-ROW",
                        )
                    ],
                )

        class LLMAdapter:
            async def complete(self, **_: object) -> str:
                return json.dumps(
                    {
                        "contracts": [
                            {
                                "contract_number": "LLM-ROW",
                                "contract_name": "LLM contract",
                                "lessee": "LLM lessee",
                                "lessor": "LLM lessor",
                                "commencement_date": "2026-01-01",
                                "lease_start_date": "2026-01-01",
                                "lease_end_date": "2028-12-31",
                                "currency": "CNY",
                                "fixed_rent_amount": 1000,
                                "payment_timing": "postpaid",
                                "suggested_scope": "in_scope",
                                "scope_confidence": 0.9,
                                "confidence": 0.9,
                            }
                        ],
                        "overall_confidence": 0.9,
                    }
                )

        result = await AIIntakeProducer().produce(
            IntakeCommand(
                kind=IntakeKind.CONTRACT_BATCH,
                file_id="file-llm-excel",
                object_name="leases.xlsx",
                content_type="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
            ),
            ExcelSourceAdapter(),
            LLMAdapter(),
        )

        self.assertEqual(result.contracts[0].contract_number, "LLM-ROW")
        self.assertFalse(result.evidence.complete)
        self.assertEqual(
            result.evidence.missing_reason,
            "field_locators_not_produced_by_llm_adapter",
        )


if __name__ == "__main__":
    unittest.main()
