CREATE TABLE "build_projects" (
	"id" uuid UNIQUE NOT NULL,
    "name" text,
    "production_branch" text,
	"created_at" timestamp,
    "build_tool" text,
    "exec_command" text,
    "workflow_file" text,
    "environment_variables" jsonb,
    "owner" uuid references users(id)
);

CREATE INDEX on build_projects (substr(name,1,20) text_pattern_ops);
