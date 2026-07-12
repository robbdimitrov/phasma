CREATE TABLE comments (
  id serial PRIMARY KEY,
  public_id uuid NOT NULL DEFAULT gen_random_uuid(),
  post_id bigint NOT NULL REFERENCES posts ON DELETE CASCADE,
  user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE,
  body varchar(400) NOT NULL CHECK (char_length(body) > 0),
  created timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX comments_post_id_created_id_idx ON comments (post_id, created DESC, id DESC);
CREATE UNIQUE INDEX comments_public_id_idx ON comments(public_id);
