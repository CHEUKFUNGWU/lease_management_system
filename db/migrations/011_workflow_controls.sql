-- +goose Up
-- +goose StatementBegin

-- Reviewers may prepare and review a close, but final approval belongs to the
-- approver role. Remove the grant even when migration 009 was already applied.
DELETE FROM permissions
WHERE role_id = '33333333-3333-3333-3333-333333333333'
  AND resource = 'monthly_closing'
  AND action = 'approve';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

INSERT INTO permissions (role_id, resource, action)
VALUES ('33333333-3333-3333-3333-333333333333', 'monthly_closing', 'approve')
ON CONFLICT (role_id, resource, action) DO NOTHING;

-- +goose StatementEnd
