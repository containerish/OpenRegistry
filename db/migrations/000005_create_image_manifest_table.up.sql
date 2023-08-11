CREATE TABLE "image_manifest" (
	"uuid" uuid PRIMARY KEY,
	"namespace" text UNIQUE NOT NULL,
	"media_type" text,
	"schema_version" int,
	"created_at" timestamp,
	"updated_at" timestamp,
  "repository_id" uuid references repository(id)
);
