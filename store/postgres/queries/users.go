package queries

var (
	AddUser = `insert into users (id, is_active, username, email, password, created_at, updated_at)
values ($1, $2, $3, $4, $5, $6, $7);`
	GetUser = `select username, is_active, email, created_at, updated_at where email = $1`
)
