CREATE TABLE "build_jobs" (
	"id" uuid UNIQUE NOT NULL,
    "logs_url" text,
    "status" text,
    "triggered_by" text,
    "duration" int,
    "branch" text,
    "commit_hash" text,
    "triggered_at" timestamp,
    "owner_id" uuid references users(id)
);
