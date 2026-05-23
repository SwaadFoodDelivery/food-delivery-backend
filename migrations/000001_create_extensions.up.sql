CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "postgis";
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'pg_partman') THEN
    EXECUTE 'CREATE EXTENSION IF NOT EXISTS "pg_partman"';
  ELSE
    RAISE NOTICE 'extension pg_partman is not available, skipping';
  END IF;
END $$;
