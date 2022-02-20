CREATE TABLE "session" (
    "id" uuid PRIMARY KEY,
    "refresh_token" text UNIQUE NOT NULL,
    "owner" uuid references users(id),
);
