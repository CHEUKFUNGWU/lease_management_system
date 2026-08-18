-- Migration 046: Store lifecycle (opening_date, closing_date, store_format)
-- and labor productivity metrics (labor_hours, headcount) for retail store-day facts.
-- Supporting SSSG comparable cohorts and labor productivity analytics (PRD F2 / Module N3 & N4).

ALTER TABLE stores
    ADD COLUMN IF NOT EXISTS opening_date DATE,
    ADD COLUMN IF NOT EXISTS closing_date DATE,
    ADD COLUMN IF NOT EXISTS store_format VARCHAR(50);

ALTER TABLE retail_store_day_facts
    ADD COLUMN IF NOT EXISTS labor_hours DECIMAL(18,2),
    ADD COLUMN IF NOT EXISTS headcount DECIMAL(18,2);

-- Check constraints for non-negative labor facts
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_retail_store_day_facts_labor_hours'
    ) THEN
        ALTER TABLE retail_store_day_facts
            ADD CONSTRAINT chk_retail_store_day_facts_labor_hours
            CHECK (labor_hours IS NULL OR labor_hours >= 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_retail_store_day_facts_headcount'
    ) THEN
        ALTER TABLE retail_store_day_facts
            ADD CONSTRAINT chk_retail_store_day_facts_headcount
            CHECK (headcount IS NULL OR headcount >= 0);
    END IF;
END $$;
