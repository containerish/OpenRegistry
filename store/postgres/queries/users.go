// nolint
package queries

var (
	AddUser = `insert into users (id, username, email, password, created_at, updated_at, is_active, webauthn_connected, github_connected, identities)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);`
	GetUser                 = `select id, is_active, username, email, created_at, updated_at, webauthn_connected, github_connected, identities from users where email=$1 or username=$1;`
	GetUserWithPassword     = `select id, is_active, username, email, password, created_at, updated_at, webauthn_connected, github_connected, identities from users where email=$1 or username=$1;`
	GetUserById             = `select id, is_active, username, email, created_at, updated_at, identities from users where id=$1;`
	GetUserByIdWithPassword = `select id, is_active, username, email, password, created_at, updated_at, identities from users where id=$1;`
	GetUserWithSession      = `select id, is_active, username, email, created_at, updated_at from users where id=(select owner from session where id=$1);`
	UpdateUser              = `update users set updated_at=$1, is_active=$2, webauthn_connected=$3, github_connected=$4, identities=$5 where id = $6;`
	SetUserActive           = `update users set is_active=true where id=$1`
	DeleteUser              = `delete from users where username = $1;`
	UpdateUserPwd           = `update users set password=$1 where id=$2;`
	GetAllEmails            = `select email from users;`
	AddOAuthUser            = `insert into users (id, username, email, github_connected, html_url, created_at, updated_at,
bio, type, gravatar_id, login, node_id, avatar_url, oauth_id, is_active, hireable)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`
	UserExists               = `select exists (select username from users where username=$1 or id=$id or login=$1 or email=$1)`
	GetGithubUser            = `select id, username, email, github_connected, webauthn_connected, identities from users where identities->'github'->>'email' = $1;`
	GetWebauthnUser          = `select id, username, email, github_connected, webauthn_connected, identities from users where identities->'webauthn'->>'username' = $1;`
	UpdateOAuthUser          = `update users set email=$1, login=$2,node_id=$3`
	UpdateUserInstallationID = `update users set github_app_installation_id=$1 where username=$2;`
	GetUserInstallationID    = `select github_app_installation_id from users where username=$1;`
)

var (
	AddSession        = `insert into session (id,refresh_token,owner) values($1, $2, (select id from users where username=$3));`
	GetSession        = `select id,refresh_token,owner from session where id=$1;`
	DeleteSession     = `delete from session where id=$1 and owner=$2;`
	DeleteAllSessions = `delete from session where owner=$1;`
)
