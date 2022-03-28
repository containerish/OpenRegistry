CREATE TABLE "sessions" (
    "id" uuid PRIMARY KEY,
    "is_active" boolean,
    "created_at" timestamp,
    "updated_at" timestamp,
    "expired_at" timestamp,
	"expires_at" timestamp,
	"refresh_token" text
);
