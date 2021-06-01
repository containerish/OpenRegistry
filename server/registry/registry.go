package registry

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/jay-dee7/parachute/skynet"
)

var contentTypes = map[string]string{
	"manifestV2Schema":     "application/vnd.docker.distribution.manifest.v2+json",
	"manifestListV2Schema": "application/vnd.docker.distribution.manifest.list.v2+json",
}

type Config struct {
	SkynetHost      string
	SkynetPortal    string
	SkynetResolvers []string
	SkynetStorePath string
}

type registry struct {
	log       *log.Logger
	blobs     blobs
	manifests manifests

	skyneStore *skynetStore

	config       *Config
	skyentClient *skynet.Client

	resolver SkynetLinkResolver
}

type restError struct {
	Status  int
	Code    string
	Message string
}

func (re *restError) Write(rw http.ResponseWriter) error {
	rw.WriteHeader(re.Status)

	return json.NewEncoder(rw).Encode(re)
}

func (r *registry) skynetURL(s []string) string {
	return fmt.Sprintf("%s/skynet/%s", r.config.SkynetPortal, strings.Join(s, "/"))
}

func (r *registry) v2(rw http.ResponseWriter, req *http.Request) *restError {
	if isBlob(req) {
		return r.blobs.handle(rw, req)
	}

	if isManifest(req) {
		return r.manifests.handle(rw, req)
	}

	rw.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	if req.URL.Path != "/v2/" && req.URL.Path != "/v2" {
		return &restError{
			Status:  http.StatusNotFound,
			Code:    "METHOD_UNKNOWN",
			Message: fmt.Sprintf("invalid api version negotiation: %s", req.URL.Path),
		}
	}

	rw.WriteHeader(200)
	return nil
}

func isDig(r *http.Request) bool {
	return r.URL.Path == "/dig/" || r.URL.Path == "/dig"
}

func (r *registry) dig(rw http.ResponseWriter, req *http.Request) {
	parser := func(s string) bool {
		if b, err := strconv.ParseBool(s); err == nil {
			return b
		}
		return false
	}

	spliter := func(s string) (string, string) {
		sa := strings.SplitN(s, ":", 2)
		if len(sa) == 1 {
			return sa[0], ""
		}

		return sa[0], sa[1]
	}

	query := req.URL.Query()
	short := parser(query.Get("short"))
	name, tag := spliter(query.Get("q"))

	if name == "" {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(rw, "Required parameter 'q' missing. /dig=?=name:tag?short=true")
		return
	}

	list := r.resolve(name, tag)
	if len(list) == 0 {
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	rw.WriteHeader(http.StatusOK)

	if tag == "" {
		skynetLink := list[0]
		rw.Header().Set("X-Docker-Content-ID", skynetLink)

		if short {
			fmt.Fprintln(rw, skynetLink)
			return
		}

		mf, err := r.manifests.getManifest(skynetLink, tag)
		if err == nil {
			fmt.Fprintln(rw, string(mf.blob))
		}
		return
	}

	for _, l := range list {
		fmt.Fprintln(rw, l)
	}
}

func (r *registry) resolve(repo, ref string) []string {
	r.log.Printf("resolving skynetLink: %s:%s", repo, ref)

	if skynetlink, ok := r.skyneStore.Get(repo, ref); ok {
		return []string{skynetlink}
	}

	// if skynetlink := ToB32(repo); skynetlink != "" {
	// 	return []string{skynetlink}
	// }

	// if hash := SkynetHash(repo); hash != "" {
	// 	if skynetlink := ToB32(hash); skynetlink != "" {
	// 		return []string{skynetlink}
	// 	}
	// }

	links := r.resolver.Resolve(repo, ref)
	defer fmt.Println(links)
	return links
}

func (r *registry) resolveSkynetLink(repo, ref string) (string, error) {
	if ref == "" {
		ref = "latest"
	}

	list := r.resolve(repo, ref)
	if len(list) > 0 {
		return list[0], nil
	}

	return "", fmt.Errorf("cannot resolve skynet link: %s:%s", repo, ref)
}

func (r *registry) root(rw http.ResponseWriter, req *http.Request) {
	if isDig(req) {
		r.dig(rw, req)
		return
	}

	if err := r.v2(rw, req); err != nil {
		err.Write(rw)
		return
	}

	r.log.Printf("%s %s", req.Method, req.URL)
}

func New(config *Config, opts ...Option) http.Handler {
	client := skynet.NewClient(&skynet.Config{
		Host:       config.SkynetHost,
		GatewayURL: config.SkynetPortal,
	})

	r := &registry{
		log:          log.New(os.Stdout, "", log.LstdFlags),
		blobs:        blobs{contents: map[string][]byte{}, uploads: map[string][]byte{}, layers: map[string][]string{}},
		manifests:    manifests{manifests: map[string]map[string]*manifest{}},
		skyneStore:   newSkynetStore(config.SkynetStorePath),
		config:       config,
		skyentClient: client,
		resolver:     nil,
	}

	r.blobs.registry = r
	r.manifests.registry = r

	r.resolver = NewResolver(client, config.SkynetResolvers)

	for _, o := range opts {
		o(r)
	}

	return http.HandlerFunc(r.root)
}

type Option func(r *registry)

func Logger(l *log.Logger) Option {
	return func(r *registry) { r.log = l }
}
