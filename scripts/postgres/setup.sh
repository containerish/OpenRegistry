#!/bin/bash

set -e
psql -v ON_ERROR_STOP=1 --host="0.0.0.0" --port="5432" --username "$PGUSER" --dbname "$PGDATABASE" <<-EOSQL
	\c open_registry;
	\i scripts/postgres/OpenRegistry.sql
EOSQL
