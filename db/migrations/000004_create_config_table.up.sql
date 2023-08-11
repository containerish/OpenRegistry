CREATE TABLE "config" (
	"uuid" uuid UNIQUE NOT NULL,
	"namespace" text NOT NULL,
	"reference" text NOT NULL,
	"digest" text NOT NULL,
	"sky_link" text,
	"media_type" text,
	"layers" text[],
	"size" int,
	"created_at" timestamp,
	"updated_at" timestamp,
  "repository_id" uuid references repository(id),
	PRIMARY KEY(namespace, reference)
);

CREATE INDEX on config (substr(namespace,1,20) text_pattern_ops);
