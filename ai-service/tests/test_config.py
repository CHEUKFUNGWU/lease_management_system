import unittest

from pydantic import ValidationError

from app.config import Settings


class PricingBookConfigTest(unittest.TestCase):
    def test_unconfigured_zero_price_book_is_valid(self):
        settings = Settings(
            llm_pricing_version="unconfigured",
            llm_input_price_usd_per_million=0,
            llm_output_price_usd_per_million=0,
        )
        self.assertEqual(settings.llm_pricing_version, "unconfigured")

    def test_configured_price_book_requires_both_positive_prices(self):
        settings = Settings(
            llm_pricing_version="deepseek-2026-08",
            llm_input_price_usd_per_million=0.14,
            llm_output_price_usd_per_million=0.28,
        )
        self.assertEqual(settings.llm_input_price_usd_per_million, 0.14)

        with self.assertRaises(ValidationError):
            Settings(
                llm_pricing_version="deepseek-2026-08",
                llm_input_price_usd_per_million=0,
                llm_output_price_usd_per_million=0.28,
            )

    def test_unconfigured_price_book_rejects_nonzero_rates(self):
        with self.assertRaises(ValidationError):
            Settings(
                llm_pricing_version="unconfigured",
                llm_input_price_usd_per_million=0.14,
                llm_output_price_usd_per_million=0,
            )

    def test_negative_price_is_rejected(self):
        with self.assertRaises(ValidationError):
            Settings(
                llm_pricing_version="unconfigured",
                llm_input_price_usd_per_million=-1,
                llm_output_price_usd_per_million=0,
            )


if __name__ == "__main__":
    unittest.main()
