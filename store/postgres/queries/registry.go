package queries

// insert queries
var (
	SetDigest = `insert into `
	SetImageManifest = `insert into image_manifest (uuid, namespace, media_type, schema_version) values ($1, $2, $3, $4);`
	SetLayer = `insert into layer (media_type, digest, sky_link, uuid, blobs, size)
	values ($1, $2, $3, $4, $5, $6);`

	SetBlob = `insert into blob (uuid, digest, sky_link, start_range, end_range)
	values ($1, $2, $3, $4, $5);`

	SetConfig = `insert into config (media_type, digest, sky_link, reference, size) 
	values ($1, $2, $3, $4, $5);`
)

// select queries
var (
	GetDigest = `select digest from layers where digest=$1;`
	ReadMetadata = `select * from metadata where namespace=$1;`
	GetLayer =`select * from layer where digest=$1;`
	GetManifest =`select * from image_manifest where namespace=$1;`
	GetBlob = `select * from blob where digest=$1;`
)

