package types

import "time"

type (
	// "id" uuid PRIMARY KEY,
	// "is_active" boolean,
	// "created_at" timestamp,
	// "updated_at" timestamp,
	// "expired_at" timestamp,
	// "expires_at" timestamp,
	// "refresh_token" text
	Session struct {
		CreatedAt    time.Time
		UpdatedAt    time.Time
		ExpiredAt    time.Time
		ExpiresAt    time.Time
		ID           string
		RefreshToken string
		IsActive     bool
	}
)
