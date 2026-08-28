POSTGRESQL_URL := 'postgres://postgres:postgres@0.0.0.0:5432/open_registry?sslmode=disable'

psql_grants:
	@psql -d open_registry -c 'GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO open_registry_user;'

mock-images:
	bash ./scripts/mock-images.sh

certs:
	mkdir .certs
	openssl req -x509 -newkey rsa:4096 -keyout .certs/registry.local -out .certs/registry.local.crt -sha256 -days 365 \
	-subj "/C=US/ST=Oregon/L=Portland/O=Company Name/OU=Org/CN=registry.local" -nodes

dummy_users:
	sh ./scripts/load_dummy_users.sh

reset:
	psql -c 'drop database open_registry' && \
		psql -c 'drop role open_registry_user' && \
		go build && \
		./OpenRegistry migrations init --database="open_registry" --admin-db="postgres" --admin-db-username="postgres" --host="0.0.0.0" --password="Qwerty@123" --insecure=true && \
		./OpenRegistry start
