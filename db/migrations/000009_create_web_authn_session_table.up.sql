CREATE TABLE "web_authn_session" (
    "challenge" text,
    "user_id" bytea,
	"credential_owner_id" uuid,
    "allowed_credential_id" bytea[],
    "user_verification" text,
    "extensions" jsonb,
	"session_type" text
)
