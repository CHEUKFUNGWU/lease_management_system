-- +goose Up
-- +goose StatementBegin

-- Records who last edited the contract.
--
-- Both ContractRepository.Update and ConfirmDiscountRate already write this
-- column, but it was never created, so every contract edit and every discount
-- rate confirmation failed with SQLSTATE 42703. Adding the column restores those
-- flows and matches the convention already used by system_settings.
ALTER TABLE lease_contracts
    ADD COLUMN IF NOT EXISTS updated_by UUID REFERENCES users(id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS updated_by;
-- +goose StatementEnd
