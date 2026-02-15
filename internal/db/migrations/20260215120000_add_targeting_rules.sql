-- +goose Up
-- +goose StatementBegin
ALTER TABLE flags
ADD COLUMN targeting_rules JSONB NOT NULL DEFAULT '[]'::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE flags DROP COLUMN targeting_rules;
-- +goose StatementEnd
