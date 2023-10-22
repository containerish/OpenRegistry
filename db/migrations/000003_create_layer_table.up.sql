CREATE TABLE "layer" (
	"uuid" uuid PRIMARY KEY,
	"digest" text UNIQUE NOT NULL,
	"blob_ids" text[],
	"media_type" text,
	"dfs_link" text,
	"size" int,
	"created_at" timestamp,
	"updated_at" timestamp
);
