DROP PUBLICATION IF EXISTS outbox_relay;
DROP INDEX IF EXISTS outbox_published_at_idx;
DROP INDEX IF EXISTS outbox_created_idx;
DROP TABLE IF EXISTS outbox;
