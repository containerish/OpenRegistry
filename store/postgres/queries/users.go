//nolint
package queries

var (
	AddUser = `insert into users (id, is_active, username, email, password, created_at, updated_at)
values ($1, $2, $3, $4, $5, $6, $7);`
	GetUser = `select id, is_active, username, email, password, created_at, updated_at from users where email=$1 
				 or username=$1;`
	UpdateUser   = `update user set username = $1, email = $2, password = $3, updated_at = $4 where username = $5;`
	DeleteUser   = `delete from user where username = $1;`
	GetAllEmails = `select email from users;`
	AddOAuthUser = `insert into users (id, username, email, password, created_at, updated_at,
bio, type, gravatar_id, login, name, node_id, avatar_url, oauth_id, is_active, hireable)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16) on conflict (email) do update set username=$2, email=$3`
)
