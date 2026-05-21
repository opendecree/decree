-- +goose Up

-- Add chain_epoch to distinguish legacy entries (epoch 0, structural-only hash)
-- from post-migration entries (epoch 1+, full payload included in hash).
-- Existing rows keep epoch 0 so VerifyChain can still validate them using the
-- original hash scheme. New inserts always use epoch 1.
ALTER TABLE audit_write_log
    ADD COLUMN chain_epoch INTEGER NOT NULL DEFAULT 0;

-- +goose Down

ALTER TABLE audit_write_log
    DROP COLUMN IF EXISTS chain_epoch;
