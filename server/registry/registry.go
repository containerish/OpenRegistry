package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jay-dee7/parachute/config"
	"github.com/jay-dee7/parachute/skynet"
	"github.com/rs/zerolog"
)

type (
	registry struct {
		l         zerolog.Logger
		blobs     blobs
		manifests manifests

		skyneStore   *skynetStore
		c            *config.RegistryConfig
		skyentClient *skynet.Client
		resolver     SkynetLinkResolver
	}

	logMsg map[string]interface{}

	restError struct {
		Status  int
		Code    string
		Message string
	}

	Option func(r *registry)
)

func (re *restError) Write(rw http.ResponseWriter) error {
	rw.WriteHeader(re.Status)

	return json.NewEncoder(rw).Encode(re)
}

func (re *restError) ToMap() map[string]interface{} {

	if re == nil {
		return map[string]interface{}{}
	}
	m := make(map[string]interface{})

	m["code"] = re.Code
	m["status"] = re.Status
	m["message"] = re.Message

	return m
}

var contentTypes = map[string]string{
	"manifestV2Schema":     "application/vnd.docker.distribution.manifest.v2+json",
	"manifestListV2Schema": "application/vnd.docker.distribution.manifest.list.v2+json",
}

func New(l zerolog.Logger, c *config.RegistryConfig, opts ...Option) http.Handler {
	client := skynet.NewClient(c)

	r := &registry{
		l: l,
		c: c,
		blobs: blobs{
			contents: map[string][]byte{},
			uploads:  map[string][]byte{},
			layers:   map[string][]string{},
			registry: &registry{},
		},
		manifests:    manifests{manifests: map[string]map[string]*manifest{}},
		skyneStore:   newSkynetStore(c.SkynetStorePath),
		skyentClient: client,
		resolver:     nil,
	}

	r.blobs.registry = r
	r.manifests.registry = r

	r.resolver = NewResolver(client, c.SkynetLinkResolvers)

	for _, o := range opts {
		o(r)
	}

	return http.HandlerFunc(r.root)
}

func Logger(l zerolog.Logger) Option {
	return func(r *registry) { r.l = l }
}

func (r *registry) skynetURL(s []string) string {
	skynetLink := s[0]
	skynetLink = strings.TrimPrefix(skynetLink, "sia://")
	uri := fmt.Sprintf("%s/%s", r.c.SkynetPortalURL, "/"+skynetLink)

	lm := make(logMsg)
	lm["uri"] = uri
	r.debugf(lm)

	return uri
}

func (r *registry) v2(rw http.ResponseWriter, req *http.Request) *restError {
	if isBlob(req) {
		err := r.blobs.handle(rw, req)
		r.debugf(err.ToMap())
		return err
	}

	if isManifest(req) {
		err := r.manifests.handle(rw, req)
		r.debugf(err.ToMap())
		return err
	}

	rw.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	if req.URL.Path != "/v2/" && req.URL.Path != "/v2" {
		err := &restError{
			Status:  http.StatusNotFound,
			Code:    "METHOD_UNKNOWN",
			Message: fmt.Sprintf("invalid api version negotiation: %s", req.URL.Path),
		}
		r.debugf(err.ToMap())
		return err
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

	lm := make(logMsg)
	if name == "" {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(rw, "Required parameter 'q' missing. /dig=?=name:tag?short=true")

		lm["error"] = "name is missing"
		r.debugf(lm)
		return
	}

	list := r.resolve(name, tag)
	if len(list) == 0 {
		lm["error"] = "status not found"
		r.debugf(lm)
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	rw.WriteHeader(http.StatusOK)

	if tag == "" {
		skynetLink := list[0]
		rw.Header().Set("X-Docker-Content-ID", skynetLink)

		if short {
			lm["skynetLink"] = skynetLink
			r.debugf(lm)
			fmt.Fprintln(rw, skynetLink)
			return
		}

		mf, err := r.manifests.getManifest(skynetLink, tag)
		if err == nil {
			lm["error"] = err.Error()
			r.debugf(lm)
			fmt.Fprintln(rw, string(mf.blob))
		}
		return
	}

	lm["list"] = list
	r.debugf(lm)
	for _, l := range list {
		fmt.Fprintln(rw, l)
	}
}

func (r *registry) resolve(repo, ref string) []string {
	lm := make(logMsg)
	lm["repo"] = repo
	lm["ref"] = ref

	if skynetlink, ok := r.skyneStore.Get(repo, ref); ok {
		lm["skynetLink"] = skynetlink
		r.debugf(lm)
		return []string{skynetlink}
	}

	links := r.resolver.Resolve(repo, ref)
	lm["links"] = links
	r.debugf(lm)
	return links
}

func (r *registry) resolveSkynetLink(repo, ref string) (string, error) {
	lm := make(logMsg)

	if ref == "" {
		ref = "latest"
	}

	list := r.resolve(repo, ref)
	if len(list) > 0 {
		lm["link"] = list[0]
		r.debugf(lm)
		return list[0], nil
	}

	err := fmt.Errorf("cannot resolve skynet link: %s:%s", repo, ref)
	lm["error"] = err.Error()
	r.debugf(lm)
	return "", err
}

func (r *registry) root(rw http.ResponseWriter, req *http.Request) {
	lm := make(logMsg)

	if isDig(req) {
		r.dig(rw, req)
		return
	}

	if err := r.v2(rw, req); err != nil {
		err.Write(rw)
		r.debugf(err.ToMap())
		return
	}

	lm["method"] = req.Method
	lm["url"] = req.URL
	r.debugf(lm)
}

func (r *registry) debugf(lm logMsg) {
	if r.c.Debug {
		e := r.l.Debug()
		e.Fields(lm).Send()
	}
}
