CREATE TABLE follows (
  follower_id bigint NOT NULL REFERENCES users ON DELETE CASCADE,
  followee_id bigint NOT NULL REFERENCES users ON DELETE CASCADE,
  created timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (follower_id, followee_id),
  CHECK (follower_id <> followee_id)
);

CREATE INDEX follows_follower_id_idx ON follows(follower_id);
CREATE INDEX follows_followee_id_idx ON follows(followee_id);
