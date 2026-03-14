CREATE TABLE recurring_rules (
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
    FOREIGN KEY (account_id) REFERENCES accounts(id)
);

CREATE TABLE transactions (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    date TEXT NOT NULL,
    amount INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    status INTEGER NOT NULL DEFAULT 1,
    recurring_rule_id TEXT,
    FOREIGN KEY (account_id) REFERENCES accounts(id)
);
