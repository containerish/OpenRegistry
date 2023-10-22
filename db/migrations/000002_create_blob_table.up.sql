CREATE TABLE "blob" (
	"uuid" uuid,
	"digest" text PRIMARY KEY,
	"dfs_link" text,
	"start_range" int,
	"end_range" int,
	"created_at" timestamp,
    "repository_id" uuid references repository(id),
);
