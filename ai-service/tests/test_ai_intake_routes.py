import json
import unittest
from io import BytesIO
from unittest.mock import AsyncMock, patch

from openpyxl import Workbook

from app.routers.parse import (
    ContractBatchDraftRequest,
    ContractDraftRequest,
    EventParseRequest,
    PaymentScheduleParseRequest,
    parse_contract,
    parse_contract_batch,
    parse_event,
    parse_payment_schedule,
)
from app.services.paddleocr import PaddleOCRClient


class VersionedAIIntakeRouteTest(unittest.IsolatedAsyncioTestCase):
    def test_paddleocr_structured_result_keeps_provider_box_not_model_box(self):
        payload = {
            "result": {
                "layoutParsingResults": [
                    {
                        "markdown": {"text": "--- OCR text ---"},
                        "prunedResult": {
                            "parsing_res_list": [
                                {"block_content": "合同编号 LEASE-001", "block_bbox": [1, 2, 31, 42]}
                            ]
                        },
                    }
                ]
            }
        }
        self.assertEqual(PaddleOCRClient._markdown_from_payload(payload), "--- Page 1 ---\n--- OCR text ---")
        locators = PaddleOCRClient._structured_locators(payload)
        self.assertEqual(len(locators), 1)
        self.assertEqual(locators[0]["page"], 1)
        self.assertEqual(locators[0]["coordinates"], [1.0, 2.0, 31.0, 42.0])
    async def test_event_route_returns_reviewable_draft_without_accounting_treatment(self):
        llm_response = {
            "choices": [
                {
                    "message": {
                        "content": json.dumps(
                            {
                                "event": {
                                    "event_type": "modification",
                                    "change_reason": "补充协议调整租金",
                                    "judgment_basis": "补充协议文本",
                                },
                                "overall_confidence": 0.7,
                                "missing_fields": ["effective_date"],
                                "warnings": ["需人工核对扫描件"],
                            }
                        )
                    }
                }
            ]
        }
        request = EventParseRequest(
            file_id="file-route-event",
            object_name="event.pdf",
            content_type="application/pdf",
            contract_id="contract-1",
        )
        with (
            patch("app.routers.parse.download_from_minio", return_value=b"pdf"),
            patch(
                "app.routers.parse.extract_text_with_evidence",
                new=AsyncMock(return_value=("event evidence", [])),
            ),
            patch(
                "app.routers.parse.llm_client.chat_completion",
                new=AsyncMock(return_value=llm_response),
            ),
        ):
            response = await parse_event(request)

        self.assertEqual(response.draft_type, "event_draft")
        self.assertEqual(response.event.contract_id, "contract-1")
        self.assertIn("effective_date", response.missing_fields)
        self.assertFalse(response.evidence.complete)
        self.assertTrue(response.review_gate.required)

    async def test_payment_schedule_route_never_guesses_missing_timing(self):
        llm_response = {
            "choices": [
                {
                    "message": {
                        "content": json.dumps(
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
                    }
                }
            ]
        }
        request = PaymentScheduleParseRequest(
            file_id="file-route-schedule",
            object_name="schedule.pdf",
            content_type="application/pdf",
        )

        with (
            patch("app.routers.parse.download_from_minio", return_value=b"pdf"),
            patch(
                "app.routers.parse.extract_text_with_evidence",
                new=AsyncMock(return_value=("schedule evidence", [])),
            ),
            patch(
                "app.routers.parse.llm_client.chat_completion",
                new=AsyncMock(return_value=llm_response),
            ),
        ):
            response = await parse_payment_schedule(request)

        self.assertEqual(response.schedules, [])
        self.assertIn("payment_timing", response.missing_fields)
        self.assertTrue(response.review_gate.required)

    async def test_contract_route_returns_versioned_assist_draft(self):
        llm_response = {
            "choices": [
                {
                    "message": {
                        "content": json.dumps(
                            {
                                "extracted_fields": {
                                    "contract_number": "LEASE-ROUTE-001",
                                    "contract_name": "测试合同",
                                    "lessee": "承租方",
                                    "lessor": "出租方",
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
                    }
                }
            ]
        }
        request = ContractDraftRequest(
            file_id="file-route-contract",
            object_name="contract.pdf",
            content_type="application/pdf",
            file_content="contract evidence",
        )

        with patch(
            "app.routers.parse.llm_client.chat_completion",
            new=AsyncMock(return_value=llm_response),
        ):
            response = await parse_contract(request)

        self.assertEqual(response.schema_version, "ai-intake.v1")
        self.assertEqual(response.draft_type, "contract_draft")
        self.assertEqual(response.extracted_data.contract_number, "LEASE-ROUTE-001")
        self.assertTrue(response.review_gate.required)
        self.assertFalse(response.evidence.complete)
        self.assertIn("discount_rate", response.missing_fields)

    async def test_contract_batch_route_returns_versioned_assist_draft(self):
        contract = {
            "contract_number": "LEASE-ROUTE-BATCH-001",
            "contract_name": "批量测试合同",
            "lessee": "承租方",
            "lessor": "出租方",
            "commencement_date": "2026-02-01",
            "lease_start_date": "2026-02-01",
            "lease_end_date": "2029-01-31",
            "currency": "CNY",
            "asset_type": "real_estate",
            "fixed_rent_amount": 2000,
            "payment_frequency": "monthly",
            "payment_timing": "postpaid",
            "discount_rate_type": "IBR",
            "discount_rate": 0.05,
            "is_lease": True,
            "suggested_scope": "in_scope",
            "scope_confidence": 0.9,
            "confidence": 0.9,
            "missing_fields": [],
            "warnings": [],
        }
        llm_response = {
            "choices": [
                {
                    "message": {
                        "content": json.dumps(
                            {
                                "contracts": [contract],
                                "total_count": 1,
                                "overall_confidence": 0.9,
                                "missing_fields": [],
                                "warnings": [],
                            }
                        )
                    }
                }
            ]
        }
        request = ContractBatchDraftRequest(
            file_id="file-route-batch",
            object_name="contracts.pdf",
            content_type="application/pdf",
            file_content="batch contract evidence",
        )

        with patch(
            "app.routers.parse.llm_client.chat_completion",
            new=AsyncMock(return_value=llm_response),
        ):
            response = await parse_contract_batch(request)

        self.assertEqual(response.schema_version, "ai-intake.v1")
        self.assertEqual(response.draft_type, "contract_batch_draft")
        self.assertEqual(response.total_count, 1)
        self.assertEqual(
            response.contracts[0].contract_number, "LEASE-ROUTE-BATCH-001"
        )
        self.assertTrue(response.review_gate.required)
        self.assertFalse(response.evidence.complete)

    async def test_excel_fallback_returns_truthful_row_evidence(self):
        workbook = Workbook()
        sheet = workbook.active
        sheet.title = "Leases"
        sheet.append([
            "合同编号",
            "合同名称",
            "法人主体",
            "出租方",
            "起租日",
            "租赁开始日",
            "租赁结束日",
            "币种",
            "月租金",
            "付款时点",
            "折现率",
            "范围判定",
        ])
        sheet.append([
            "LEASE-EXCEL-001",
            "Excel 合同",
            "承租方",
            "出租方",
            "2026-03-01",
            "2026-03-01",
            "2029-02-28",
            "CNY",
            3000,
            "后付",
            0.05,
            "in_scope",
        ])
        workbook_bytes = BytesIO()
        workbook.save(workbook_bytes)
        workbook.close()

        llm_response = {
            "choices": [
                {
                    "message": {
                        "content": json.dumps(
                            {
                                "contracts": [],
                                "total_count": 0,
                                "overall_confidence": 0,
                                "missing_fields": [],
                                "warnings": [],
                            }
                        )
                    }
                }
            ]
        }
        request = ContractBatchDraftRequest(
            file_id="file-route-excel",
            object_name="lease-ledger.xlsx",
            content_type="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        )

        with (
            patch(
                "app.routers.parse.download_from_minio",
                return_value=workbook_bytes.getvalue(),
            ),
            patch(
                "app.routers.parse.llm_client.chat_completion",
                new=AsyncMock(return_value=llm_response),
            ),
        ):
            response = await parse_contract_batch(request)

        self.assertEqual(response.contracts[0].contract_number, "LEASE-EXCEL-001")
        self.assertTrue(response.evidence.complete)
        self.assertEqual(response.evidence.locators[0].field, "contracts[0]")
        self.assertEqual(response.evidence.locators[0].source, "Leases!A2:L2")


if __name__ == "__main__":
    unittest.main()
