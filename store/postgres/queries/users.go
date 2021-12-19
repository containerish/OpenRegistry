//nolint
package queries

var (
	AddUser = `insert into users (id, is_active, username, email, password, created_at, updated_at)
values ($1, $2, $3, $4, $5, $6, $7);`
	GetUser = `select is_active, username, email, password, created_at, updated_at from users where email=$1 
				 or username=$1;`
	UpdateUser   = `update user set username = $1, email = $2, password = $3, updated_at = $4 where username = $5;`
	DeleteUser   = `delete from user where username = $1;`
	GetAllEmails = `select email from users;`
)
