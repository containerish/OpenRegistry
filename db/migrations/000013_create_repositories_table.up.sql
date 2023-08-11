CREATE TABLE "repositories" (
	"uuid" uuid PRIMARY KEY,
  "name" text,
  "owner" uuid references users(id),
  "visibility" text,
  "description" text
);
