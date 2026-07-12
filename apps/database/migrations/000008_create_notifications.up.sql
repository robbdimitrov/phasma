CREATE TABLE notifications (
  id bigserial PRIMARY KEY,
  public_id uuid NOT NULL DEFAULT gen_random_uuid(),
  external_id varchar(255) NOT NULL,
  user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  actor_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type varchar(20) NOT NULL CHECK (type IN ('like', 'comment', 'follow')),
  entity_id varchar(255) NOT NULL,
  read bool NOT NULL DEFAULT false,
  created timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX notifications_external_id_idx ON notifications(external_id);
CREATE UNIQUE INDEX notifications_public_id_idx ON notifications(public_id);
CREATE INDEX notifications_user_id_created_idx ON notifications(user_id, created DESC, id DESC);
CREATE INDEX notifications_type_entity_id_idx ON notifications(type, entity_id);
CREATE INDEX notifications_user_id_unread_idx ON notifications(user_id) WHERE read = false;
