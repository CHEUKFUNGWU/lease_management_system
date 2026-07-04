import json
import unittest
from pathlib import Path

from app.intake.models import (
    ContractBatchIntakeResponse,
    ContractIntakeResponse,
    PaymentScheduleIntakeResponse,
    PaymentScheduleItem,
    build_payment_schedule_intake,
)


class PaymentScheduleIntakeContractTest(unittest.TestCase):
    def test_shared_v1_fixture_matches_python_contract(self):
        fixture_path = (
            Path(__file__).resolve().parents[2]
            / "contracts"
            / "ai-intake.v1"
            / "payment-schedule.json"
        )
        payload = json.loads(fixture_path.read_text(encoding="utf-8"))

        intake = PaymentScheduleIntakeResponse.model_validate(payload)

        self.assertEqual(intake.schema_version, "ai-intake.v1")
        self.assertTrue(intake.evidence.complete)
        self.assertTrue(intake.review_gate.required)

    def test_complete_evidence_must_cover_every_schedule(self):
        fixture_path = (
            Path(__file__).resolve().parents[2]
            / "contracts"
            / "ai-intake.v1"
            / "payment-schedule.json"
        )
        payload = json.loads(fixture_path.read_text(encoding="utf-8"))
        payload["evidence"]["locators"][0]["field"] = "unrelated_field"

        with self.assertRaisesRegex(ValueError, "does not cover schedules\\[0\\]"):
            PaymentScheduleIntakeResponse.model_validate(payload)

    def test_builder_marks_missing_adapter_evidence_for_review(self):
        intake = build_payment_schedule_intake(
            file_id="file-002",
            object_name="schedule.xlsx",
            content_type="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
            schedules=[
                PaymentScheduleItem(
                    period_start="2026-02-01",
                    period_end="2026-02-28",
                    due_date="2026-02-01",
                    amount=1000,
                    payment_timing="prepaid",
                    is_fixed=True,
                    is_lease_component=True,
                    confidence=0.7,
                )
            ],
            confidence_scores={"overall": 0.7, "average_item": 0.7},
            missing_fields=[],
            warnings=[],
        )

        self.assertFalse(intake.evidence.complete)
        self.assertEqual(
            intake.evidence.missing_reason,
            "field_locators_not_produced_by_adapter",
        )
        self.assertIn("low_confidence", intake.review_gate.reasons)
        self.assertIn("evidence_incomplete", intake.review_gate.reasons)


class ContractIntakeContractTest(unittest.TestCase):
    def test_shared_contract_fixture_matches_python_contract(self):
        fixture_path = (
            Path(__file__).resolve().parents[2]
            / "contracts"
            / "ai-intake.v1"
            / "contract.json"
        )
        payload = json.loads(fixture_path.read_text(encoding="utf-8"))

        intake = ContractIntakeResponse.model_validate(payload)

        self.assertEqual(intake.draft_type, "contract_draft")
        self.assertEqual(intake.extracted_data.contract_number, "LEASE-001")
        self.assertFalse(intake.evidence.complete)
        self.assertIn("discount_rate", intake.missing_fields)


class ContractBatchIntakeContractTest(unittest.TestCase):
    def test_shared_contract_batch_fixture_matches_python_contract(self):
        fixture_path = (
            Path(__file__).resolve().parents[2]
            / "contracts"
            / "ai-intake.v1"
            / "contract-batch.json"
        )
        payload = json.loads(fixture_path.read_text(encoding="utf-8"))

        intake = ContractBatchIntakeResponse.model_validate(payload)

        self.assertEqual(intake.draft_type, "contract_batch_draft")
        self.assertEqual(intake.total_count, len(intake.contracts))
        self.assertEqual(intake.contracts[0].contract_number, "LEASE-BATCH-001")
        self.assertTrue(intake.evidence.complete)
        self.assertEqual(intake.evidence.locators[0].source, "Leases!A2:Z2")


if __name__ == "__main__":
    unittest.main()
