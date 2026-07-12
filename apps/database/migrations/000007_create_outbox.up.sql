CREATE TABLE outbox (
  id bigserial PRIMARY KEY,
  topic varchar(50) NOT NULL,
  payload jsonb NOT NULL,
  published_at timestamptz,
  created timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX outbox_created_idx ON outbox(created);
CREATE INDEX outbox_published_at_idx ON outbox(published_at) WHERE published_at IS NULL;

CREATE PUBLICATION outbox_relay FOR TABLE outbox;
