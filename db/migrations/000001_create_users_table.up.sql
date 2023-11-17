CREATE TABLE "users" (
	"id" uuid PRIMARY KEY,
	"username" varchar NOT NULL UNIQUE,
	"email" varchar NOT NULL UNIQUE,
	"password" varchar,
	"created_at" timestamp,
	"updated_at" timestamp,
	"is_active" boolean,  
    "webauthn_connected" boolean default false,
    "github_connected" boolean default false,
    "identities" jsonb default '{}'
);
