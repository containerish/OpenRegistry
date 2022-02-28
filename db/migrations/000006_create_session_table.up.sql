CREATE TABLE "session" (
    "id" uuid PRIMARY KEY,
    "is_active" boolean,
    "expires_at" timestamp,
    "refresh_token" text UNIQUE NOT NULL,
    "owner" uuid references users(id)
);
