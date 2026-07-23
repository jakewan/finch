-- Mark transaction rows that are one leg of a transfer between two accounts.
-- Cash flow needs to distinguish them: moving money between your own accounts is
-- neither income nor an expense, and without this column a recorded transfer is
-- indistinguishable from ordinary activity.
ALTER TABLE transactions ADD COLUMN is_transfer INTEGER NOT NULL DEFAULT 0;

-- Backfill from the event log, which already links both legs of every transfer
-- through TransferCreated. The read model stays derivable from events.
UPDATE transactions SET is_transfer = 1 WHERE id IN (
    SELECT json_extract(payload, '$.source_transaction_id') FROM events
    WHERE event_type = 'TransferCreated'
    UNION ALL
    SELECT json_extract(payload, '$.dest_transaction_id') FROM events
    WHERE event_type = 'TransferCreated'
);

-- Add the foreign key that recurring_rules.transfer_target_account_id has been
-- missing. Projection now moves money through this column, so referential integrity
-- belongs in the database rather than in application checks alone. SQLite cannot add
-- a constraint in place, so the table is rebuilt. No other table references
-- recurring_rules, so the drop breaks no constraint.
CREATE TABLE recurring_rules_new (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    name TEXT NOT NULL,
    amount INTEGER NOT NULL,
    frequency INTEGER NOT NULL,
    start_date TEXT NOT NULL,
    end_date TEXT,
    day_of_month INTEGER,
    semi_monthly_days TEXT,
    is_transfer INTEGER NOT NULL DEFAULT 0,
    transfer_target_account_id TEXT,
    paused INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    FOREIGN KEY (transfer_target_account_id) REFERENCES accounts(id),
    -- The states projection cannot honor are rejected by the database, not only by
    -- the Go write paths. A transfer rule reaching projection with a null target
    -- would be read as an ordinary signed amount and booked as income on its source
    -- account, and one with a non-positive amount would move money backwards. The
    -- foreign key above permits NULL, so it does not cover the first case.
    CHECK (is_transfer = 0 OR transfer_target_account_id IS NOT NULL),
    CHECK (is_transfer = 0 OR amount > 0),
    CHECK (transfer_target_account_id IS NULL OR transfer_target_account_id <> account_id)
);

INSERT INTO recurring_rules_new
SELECT id, account_id, name, amount, frequency, start_date, end_date,
       day_of_month, semi_monthly_days, is_transfer, transfer_target_account_id, paused
FROM recurring_rules;

DROP TABLE recurring_rules;

ALTER TABLE recurring_rules_new RENAME TO recurring_rules;
