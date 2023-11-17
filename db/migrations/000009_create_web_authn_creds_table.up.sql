CREATE TABLE "web_authn_creds" (
	"credential_owner_id" uuid,
    "id" bytea,
    "public_key" bytea,
    "attestation_type" text,
    "aaguid" bytea,
    "sign_count" integer,
    "clone_warning" bool
)
