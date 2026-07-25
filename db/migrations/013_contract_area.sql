-- +goose Up
-- +goose StatementBegin

-- Leased area, in square metres.
--
-- It lives on the contract rather than on the store because rent is paid for the
-- area named in that lease: one store can be covered by several leases, and a
-- store's own gross area is different master data. Leases of vehicles, IT
-- equipment and machinery simply leave it null, which keeps them out of any
-- per-square-metre comparison instead of distorting it.
ALTER TABLE lease_contracts
    ADD COLUMN IF NOT EXISTS area_sqm DECIMAL(12, 2);

ALTER TABLE lease_contracts
    ADD CONSTRAINT lease_contracts_area_sqm_positive
    CHECK (area_sqm IS NULL OR area_sqm > 0);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE lease_contracts DROP CONSTRAINT IF EXISTS lease_contracts_area_sqm_positive;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS area_sqm;
-- +goose StatementEnd
