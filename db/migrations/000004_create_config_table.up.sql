CREATE TABLE "config" (
     "uuid" uuid UNIQUE NOT NULL,
     "namespace" text NOT NULL,
     "reference" text NOT NULL,
     "digest" text NOT NULL,
     "sky_link" text,
     "media_type" text,
     "layers" text[],
     "size" int,
     PRIMARY KEY(namespace, reference)
);

