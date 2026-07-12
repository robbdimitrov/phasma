REVOKE SELECT ON post_hashtags FROM phasma_connect;
REVOKE SELECT ON hashtags FROM phasma_connect;
REVOKE SELECT ON posts FROM phasma_connect;
REVOKE SELECT ON users FROM phasma_connect;
REVOKE SELECT ON outbox FROM phasma_connect;
REVOKE USAGE ON SCHEMA public FROM phasma_connect;
REVOKE CONNECT ON DATABASE phasma FROM phasma_connect;
DO $$ BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'phasma_connect_user') THEN
    REVOKE phasma_connect FROM phasma_connect_user;
  END IF;
END $$;
DROP ROLE IF EXISTS phasma_connect;
