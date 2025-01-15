# Get started with OpenRegistry Development on Linux/MacOS

## Clone the repository

We recommend using the Git method to clone the repository:

```bash
git clone git@github.com:containerish/OpenRegistry.git
cd OpenRegistry
```

## Configuration File

OpenRegistry uses the standard yaml based configuration. This configuration file is named `config.yaml` and can be
either in the current directory or `$HOME/.openregistry/config.yaml` directory. Some of the features are disabled by
default just to keep the on-boarding process simple.

- Add the following contents to `config.yaml` file:

```yaml
environment: local
debug: true
web_app_url: "http://localhost:3000"
web_app_redirect_url: "/"
web_app_error_redirect_path: "/auth/unhandled"
registry:
  dns_address: registry.local
  version: master
  fqdn: registry.local
  host: registry.local
  port: 5000
  tls:
    enabled: true
    key: .certs/openregistry.key
    cert: .certs/openregistry.cert
  services:
    - github
    - token
dfs:
  s3_any:
    access_key: <access-key>
    secret_key: <access-secret-key>
    endpoint: <s3-compatible-api-endpoint>
    bucket_name: <s3-bucket-name>
    dfs_link_resolver: <optional-dfs-link-resolver-url>
database:
  kind: postgres
  host: 0.0.0.0
  port: 5432
  username: postgres
  password: Qwerty@123
  name: open_registry
```

If you check the `registry.tls` section, you'll notice that we have enabled the TLS configuration, but we need to
generate the TLS certificates before we move forward:

```bash
just certs
```

## Database Setup

We currently use PostgreSQL, any modern version of Postgres should work fine but we recommend using the latest version.

- [Install PostgreSQL on Ubuntu](https://www.digitalocean.com/community/tutorials/how-to-install-postgresql-on-ubuntu-22-04-quickstart)
- [Install PostgreSQL on MacOS With Homebrew](https://formulae.brew.sh/formula/postgresql@15)
- [For MacOS, there's Postgres APP as well](https://postgresapp.com/)

### Create Database and tables

Once you've installed PostgreSQL, login to Postgres Shell and create a database:

```bash
psql
>CREATE DATABASE open_registry;
```

Exit the Postgres shell.

### Create the tables

We have a simple Makefile, which exposes the following commands:

- `migup` - Populate all the migrations, create tables, schema changes
- `migdown` - Teardown all the tables, schemas, etc
- `cleanup` - Runs `migdown` first and then `migup`

Before we begin setting up tables in our database, we need to use another tool called `golang-migrate`.
This is a database migration tool that makes database migrations dead simple. Use either of the following links to
install `golang-migrate`:

- [Homebrew Link](https://formulae.brew.sh/formula/golang-migrate#default)
- [GitHub Release](https://github.com/golang-migrate/migrate/releases)

To make sure that OpenRegistry can find all the required tables, schemas, etc, run the following command:

```bash
just migup
```

```bash
Output:
migrate -database 'postgres://postgres:postgres@0.0.0.0:5432/open_registry?sslmode=disable' -path db/migrations up
1/u create_users_table (9.436292ms)
2/u create_blob_table (17.848625ms)
3/u create_layer_table (24.613917ms)
4/u create_config_table (29.878583ms)
5/u create_image_manifest_table (33.47625ms)
6/u create_session_table (37.176292ms)
7/u create_verify_emails_table (40.206583ms)
```

## Run OpenRegistry

```bash
go build
./OpenRegistry
```

```bash
Output:
connection to database successful
Environment: LOCAL
Service Endpoint: https://registry.local:5000
⇨ https server started on 192.168.1.3:5000
```
