package queries

// insert queries
var (
	SetDigest        = `insert into `
	SetImageManifest = `insert into image_manifest (uuid, namespace, media_type, schema_version) 
	values ($1, $2, $3, $4) on conflict (namespace) do update set schema_version=3`
	SetLayer = `insert into layer (media_type, digest, sky_link, uuid, blob_ids, size)
	values ($1, $2, $3, $4, $5, $6) on conflict (digest) do nothing;`

	// SetBlob TODO - (guacamole/jay-dee7) find a better way to handle duplicates in blob
	SetBlob = `insert into blob (uuid, digest, sky_link, start_range, end_range)
	values ($1, $2, $3, $4, $5) on conflict (digest) do nothing;`

	SetConfig = `insert into config (uuid, namespace, reference, digest, sky_link, media_type, layers, size) 
	values ($1, $2, $3, $4, $5, $6,$7, $8) on conflict (namespace,reference) 
	do update set digest=$4, sky_link=$5,layers=$7;`
)

// select queries
var (
	GetDigest        = `select digest from layers where digest=$1;`
	ReadMetadata     = `select * from metadata where namespace=$1;`
	GetLayer         = `select * from layer where digest=$1;`
	GetManifest      = `select * from image_manifest where namespace=$1;`
	GetBlob          = `select * from blob where digest=$1;`
	GetConfig        = `select * from config where namespace=$1;`
	GetImageTags     = `select reference from config where namespace=$1;`
	GetManifestByRef = `select * from config where namespace=$1 and reference=$2;`
	GetManifestByDig = `select * from config where namespace=$1 and digest=$2;`
	GetCatalog       = `select namespace,reference,digest from config;`
)

// delete queries
var (
	DeleteLayer         = `delete from layer where digest=$1;`
	DeleteBlob          = `delete from blob where digest=$1;`
	DeleteManifestByRef = `delete from config where reference=$1;`
	DeleteManifestByDig = `delete from config where digest=$1;`
)
