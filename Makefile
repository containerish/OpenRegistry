POSTGRESQL_URL='postgres://postgres:postgres@0.0.0.0:5432/open_registry?sslmode=disable'

migup:
	migrate -database ${POSTGRESQL_URL} -path db/migrations up
migdown:
	migrate -database ${POSTGRESQL_URL} -path db/migrations down

cleanup: migdown migup

mock-images:
	bash ./scripts/mock-images.sh

tools:
	pip3 install ggshield pre-commit
	pre-commit install

