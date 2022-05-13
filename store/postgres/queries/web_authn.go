package queries

var (
	AddWebAuthNSessionData = `insert into web_authn_session (challenge,user_id,allowed_credential_id,
	user_verification,extensions) values ($1,$2,$3,$4,$5);`
	GetWebAuthNSessionData = `select * from web_authn_session where user_id=$1;`

	AddWebAuthNCredentials = `insert into web_authn_creds (id,public_key,attestation_type,aaguid,
	sign_count,clone_warning) values ($1,$2,$3,$4,$5,$6);`
	GetWebAuthNCredentials = `select * from web_authn_creds where id=$1;`
)
