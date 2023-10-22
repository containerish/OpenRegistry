// nolint
package queries

// insert queries
var (
	SetImageManifest = `insert into image_manifest (uuid, namespace, media_type, schema_version, created_at, updated_at) 
	values ($1, $2, $3, $4, $5, $6) on conflict (namespace) do update set updated_at=$6`
	SetLayer = `insert into layer (media_type, digest, dfs_link, uuid, blob_ids, size,created_at,updated_at)
	values ($1, $2, $3, $4, $5, $6,$7,$8) on conflict (digest) do update set updated_at=$8;`

	// SetBlob TODO - (guacamole/jay-dee7) find a better way to handle duplicates in blob
	SetBlob = `insert into blob (uuid, digest, dfs_link, start_range, end_range, created_at)
	values ($1, $2, $3, $4, $5, $6) on conflict (digest) do nothing;`

	SetConfig = `insert into config (uuid, namespace, reference, digest, dfs_link, media_type, layers, size,
	created_at, updated_at) values ($1, $2, $3, $4, $5, $6,$7, $8, $9, $10) on conflict (namespace,reference) 
	do update set digest=$4, dfs_link=$5,layers=$7,updated_at=$10;`
)

// select queries
var (
	GetDigest                    = `select digest from layers where digest=$1;`
	ReadMetadata                 = `select * from metadata where namespace=$1;`
	GetLayer                     = `select * from layer where digest=$1;`
	GetContentHashById           = `select dfs_link from layer where uuid=$1;`
	GetManifest                  = `select * from image_manifest where namespace=$1;`
	GetBlob                      = `select * from blob where digest=$1;`
	GetConfig                    = `select * from config where namespace=$1;`
	GetImageTags                 = `select reference from config where namespace=$1;`
	GetManifestByRef             = `select * from config where namespace=$1 and reference=$2;`
	GetManifestByDig             = `select * from config where namespace=$1 and digest=$2;`
	GetCatalogCount              = `select count(namespace) from image_manifest;`
	GetUserCatalogCount          = `select count(namespace) from image_manifest where namespace like $1;`
	GetCatalog                   = `select namespace from image_manifest;`
	GetCatalogWithPagination     = `select namespace from image_manifest limit $1 offset $2;`
	GetUserCatalogWithPagination = `select namespace from image_manifest where namespace like $1 limit $2 offset $3;`
	GetImageNamespace            = `select uuid,namespace,created_at::timestamptz,updated_at::timestamptz from 
		image_manifest where substr(namespace, 1, 50) like $1;`

	// be very careful using this one
	GetCatalogDetailWithPagination = `select namespace,created_at::timestamptz,updated_at::timestamptz from
	image_manifest order by %s limit $1 offset $2;`
	GetUserCatalogDetailWithPagination = `select namespace,created_at::timestamptz,updated_at::timestamptz from 
		image_manifest where namespace like $1 order by %s limit $2 offset $3;`
	GetRepoDetailWithPagination = `select reference, digest, dfs_link, (select sum(size) from layer where digest = 
		ANY(layers)) as size, created_at::timestamptz, updated_at::timestamptz from config where namespace=$1 
		limit $2 offset $3;`
	UpdateContainerImageVisibility = `update image_manifest set visibility=$1 where uuid=$2`
)

// delete queries
var (
	DeleteLayer         = `delete from layer where digest=$1;`
	DeleteBlob          = `delete from blob where digest=$1;`
	DeleteManifestByRef = `delete from config where reference=$1;`
	DeleteManifestByDig = `delete from config where digest=$1;`
)
