CREATE TABLE "repositori" (
	"uuid" uuid,
	"digest" text PRIMARY KEY,
	"sky_link" text,
	"start_range" int,
	"end_range" int,
	"created_at" timestamp
);
