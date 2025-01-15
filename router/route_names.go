package router

const (
	//V2 endpoint suggests that we support Distribution spec's HTTP2 API
	V2 = "/v2"

	// Namespace endpoint refers to a single repository under a particular user
	Namespace = "/:username/:imagename"

	// Internal endpoint refers to the internal APIs not supposed to be exposed
	Internal = "/internal"

	// Auth endpoint Authenticates user through basic auth or any supported
	// authentication mechanisms
	Auth = "/auth"

	Webauthn = Auth + "/webauthn"

	GitHub = "/github"
	//Beta endpoint refers to the experimental code and features under observation
	// not to be released or exposed to public
	Beta = "/beta"

	//Root is the root path for entire application
	Root = "/"

	//BlobsDigest endpoint exposes the binary form of content stored by registry, addressed by digest
	//Digests are unique identifiers created from a cryptographic hash of a Blob's content
	//used by methods: LayerExists, PullLayer, DeleteLayer
	BlobsDigest = "/blobs/:digest"

	//ManifestsReference endpoint is a reference to the json document which defines an artifact
	//used by methods: ManifestExists, PushManifest, PullManifest, DeleteTagOrManifest
	ManifestsReference = "/manifests/:reference"

	//BlobsUploads endpoint is used to start and complete blob uploads to the registry
	//by the methods : StartUpload and CompleteUpload
	BlobsUploads = "/blobs/uploads/"

	// Blobs Upload an image monolithically with a POST+PUT Operation
	BlobsMonolithicPut = "/blobs/monolithic/upload/:uuid"

	//BlobsUploadsUUID serves similar functionality within an upload
	// i.e. layered and chunked uploads by methods: PushLayer, ChunkedUpload, CompleteUpload, UploadProgress
	BlobsUploadsUUID = BlobsUploads + ":uuid"

	// TagsList endpoint is used to list the tags attached to images, e.g. latest, alpine , etc
	// this is also a part of catalog api
	TagsList = "/tags/list"

	GetReferrers = "/referrers/:digest"

	// Catalog is used to list the available repositories
	Catalog = "/_catalog"

	// PublicCatalog is used to list the available public repositories
	PublicCatalog = "/_catalog/public"

	// Prefix for Extensions
	Ext = "/ext"

	// Catalog Extensions API Prefix
	C = Ext + "/catalog"

	// JWT based auth endpoint
	TokenAuth = "/token"
	Search    = C + "/search"

	// API to get detailed catalog information
	CatalogDetail = C + "/detail"
	UserCatalog   = C + "/user"

	RepositoryDetail = C + "/repository"

	ChangeRepositoryVisibility = Ext + "/repository/visibility"
	CreateRepository           = Ext + "/repository/create"
	RepositoryFavorites        = Ext + "/repository/favorites"
	RemoveRepositoryFavorites  = Ext + "/repository/favorites/:repository_id"
)
