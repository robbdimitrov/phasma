CREATE TABLE audit_log (
  id bigserial PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE,
  action varchar(50) NOT NULL,
  created timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_user_id_idx ON audit_log(user_id);
