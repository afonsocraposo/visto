ALTER TABLE notification_deliveries ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 1
    CHECK (attempt_count > 0);
ALTER TABLE notification_deliveries ADD COLUMN next_attempt_at TEXT;

UPDATE notification_deliveries
SET next_attempt_at=attempted_at
WHERE state='failed';
