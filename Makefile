POSTGRESQL_URL='postgres://postgres:postgres@0.0.0.0:5432/open_registry?sslmode=disable'

migup: put-pg-uuid-v7
	migrate -database ${POSTGRESQL_URL} -path db/migrations up
migdown:
	migrate -database ${POSTGRESQL_URL} -path db/migrations down

put-pg-uuid-v7:
	curl -sSL https://gist.githubusercontent.com/jay-dee7/62fb7f665101a52c9c27dcff5bad03b6/raw/3fc979ead02ecbaba256819d309b2a9768a9d5b8/pg-uuid-v7.sql > /tmp/pg-uuid-v7.sql
	/usr/local/bin/psql -U postgres -d open_registry -f /tmp/pg-uuid-v7.sql
cleanup: migdown migup put-pg-uuid-v7

mock-images:
	bash ./scripts/mock-images.sh

tools:
	pip3 install ggshield pre-commit
	pre-commit install

