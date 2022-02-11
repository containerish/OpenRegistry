package queries

var (
	//nolint
	AddSession = `insert into sessions (id, is_active, created_at, expires_at, refresh_token)
	values ($1, $2, $3, $4, $5);`
	//nolint
	UpdateSession = `update sessions set is_active=$2, updated_at=$3, expired_at=$4 where id=$1`
	//nolint
	ExpireSession = `update sessions set is_active=false, updated_at=NOW(), expired_at=NOW() where id=$1`
)
