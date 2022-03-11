package queries

var (
	AddVerifyUser    = `insert into verify_emails (token,user_id) values ($1,$2);`
	GetVerifyUser    = `select token from verify_emails where user_id=$1;`
	DeleteVerifyUser = `delete from verify_emails where user_id=$1;`
)
