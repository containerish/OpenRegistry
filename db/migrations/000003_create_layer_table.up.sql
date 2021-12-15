CREATE TABLE "layer" (
     "uuid" uuid PRIMARY KEY,
     "digest" text UNIQUE NOT NULL,
     "blob_ids" text[],
     "media_type" text,
     "sky_link" text,
     "size" int
);
