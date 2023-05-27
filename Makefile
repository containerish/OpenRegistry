POSTGRESQL_URL='postgres://postgres:postgres@0.0.0.0:5432/open_registry?sslmode=disable'

migup:
	@migrate -database ${POSTGRESQL_URL} -path db/migrations up
migdown:
	@migrate -database ${POSTGRESQL_URL} -path db/migrations down
psql_grants:
	@psql -d open_registry -c 'GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO open_registry_user;'

cleanup: migdown migup psql_grants

mock-images:
	bash ./scripts/mock-images.sh

tools:
	pip3 install ggshield pre-commit
	pre-commit install

certs:
	mkdir .certs
	openssl req -x509 -newkey rsa:4096 -keyout .certs/registry.local -out .certs/registry.local.crt -sha256 -days 365 \
	-subj "/C=US/ST=Oregon/L=Portland/O=Company Name/OU=Org/CN=registry.local" -nodes
