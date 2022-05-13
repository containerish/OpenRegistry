CREATE TABLE "web_authn_creds" (
    "id" text,
    "public_key" text,
    "attestation_type" text,
    "aaguid" bytea,
    "sign_count" integer,
    "clone_warning" bool
)
