#!/bin/bash

set -e
/Applications/Postgres.app/Contents/Versions/14/bin/psql -v ON_ERROR_STOP=1 --username "$PGUSER" --dbname "$PGDATABASE" <<-EOSQL
	CREATE USER jane_doe;
    GRANT ALL PRIVILEGES ON DATABASE open_registry TO jane_doe;
	\c open_registry;
	\i scripts/postgres/OpenRegistry.sql
EOSQL
