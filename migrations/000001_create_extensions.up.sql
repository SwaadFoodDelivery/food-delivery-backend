CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "postgis";
DO $$
BEGIN
  CREATE EXTENSION IF NOT EXISTS "pg_partman";
EXCEPTION
  WHEN undefined_file THEN
    RAISE NOTICE 'extension pg_partman is not available, skipping';
END $$;
