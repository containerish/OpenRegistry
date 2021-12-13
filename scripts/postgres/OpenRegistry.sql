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

CREATE TABLE "config" (
  "uuid" uuid UNIQUE NOT NULL,
  "namespace" text,
  "reference" text,
  "digest" text UNIQUE NOT NULL,
  "sky_link" text,
  "media_type" text,
  "layers" text[],
  "size" int,
  PRIMARY KEY (namespace,reference)
);

CREATE TABLE "blob" (
  "uuid" uuid,
  "digest" text PRIMARY KEY,
  "sky_link" text,
  "start_range" int,
  "end_range" int
);

CREATE TABLE "layer" (
  "uuid" uuid PRIMARY KEY,
  "digest" text UNIQUE NOT NULL,
  "blob_ids" text[],
  "media_type" text,
  "sky_link" text,
  "size" int
);

CREATE TABLE "image_manifest" (
  "uuid" uuid PRIMARY KEY,
  "namespace" text UNIQUE NOT NULL,
  "media_type" text,
  "schema_version" int
);
