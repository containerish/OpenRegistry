package queries

var (
	// user_id is the web_authn_session user_id
	// credential_owner_id is from our user table
	AddWebAuthNSessionData = `insert into web_authn_session (credential_owner_id,user_id,challenge,allowed_credential_id,
	user_verification,extensions,session_type) values ($1,$2,$3,$4,$5,$6,$7) on conflict (credential_owner_id) do update set user_id=$2,challenge=$3,allowed_credential_id=$4,user_verification=$5,extensions=$6,session_type=$7;`
	GetWebAuthNSessionData = `select user_id,challenge,allowed_credential_id,user_verification,extensions from
	web_authn_session where credential_owner_id=$1 and session_type=$2;`

	AddWebAuthNCredentials = `insert into web_authn_creds (credential_owner_id,id,public_key,attestation_type,aaguid,
	sign_count,clone_warning) values ($1,$2,$3,$4,$5,$6,$7);`
	GetWebAuthNCredentials = `select id,public_key,attestation_type,aaguid,sign_count,clone_warning from web_authn_creds
	where credential_owner_id=$1;`
)
