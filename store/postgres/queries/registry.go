package queries

// insert queries
var (
	SetMetadata = `insert into metadata (uuid, namespace, manifest) values($1, $2, $3);`
	SetImageManifest = `insert into image_manifest (uuid, media_type, layers, config,
	schema_version) values ($1, $2, $3, $4, $5);`
	SetLayer = `insert into layer (media_type, digest, sky_link, uuid, blobs, size)
	values ($1, $2, $3, $4, $5, $6);`
	SetBlob = `insert into blob (uuid, digest, sky_link, start_range, end_range)
	values ($1, $2, $3, $4, $5);`
	SetConfig = `insert into config (media_type, digest, sky_link, reference, size) 
	values ($1, $2, $3, $4, $5);`
)

// select queries
var (
	GetAllEmails = `select email from users;`
	ReadMetadata = `select * from metadata where namespace=$1;`
)

