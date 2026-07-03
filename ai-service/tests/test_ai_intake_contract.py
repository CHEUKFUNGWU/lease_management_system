import json
import unittest
from pathlib import Path

from app.intake.models import (
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


if __name__ == "__main__":
    unittest.main()
