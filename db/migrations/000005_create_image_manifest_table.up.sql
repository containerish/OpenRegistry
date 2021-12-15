CREATE TABLE "image_manifest" (
     "uuid" uuid PRIMARY KEY,
     "namespace" text UNIQUE NOT NULL,
     "media_type" text,
     "schema_version" int
);

