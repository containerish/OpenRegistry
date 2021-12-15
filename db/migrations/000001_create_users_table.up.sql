CREATE TABLE "users" (
    "id" uuid PRIMARY KEY,
    "is_active" boolean,
    "username" varchar NOT NULL UNIQUE,
    "email" varchar NOT NULL UNIQUE,
    "password" varchar,
    "created_at" timestamp,
    "updated_at" timestamp,
    "country_code" int
);
