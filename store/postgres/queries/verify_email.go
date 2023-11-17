package queries

const (
	AddVerifyUser    = `insert into verify_emails (token,user_id) values ($1,$2);`
	GetVerifyUser    = `select user_id from verify_emails where token=$1;`
	DeleteVerifyUser = `delete from verify_emails where token=$1;`
)
