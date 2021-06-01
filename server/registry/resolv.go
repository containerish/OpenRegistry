package registry

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jay-dee7/parachute/skynet"
)


type SkynetLinkResolver interface {
	Resolve(repo string, refernce string) []string
}

func lookup(domain string) (string, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if !strings.HasPrefix(domain, "_dnslink.") {
		domain = "_dnslink." + domain
	}

	txts, err := net.LookupTXT(domain)
	if err != nil {
		return "", err
	}

	for _, txt := range txts {
		if txt != "" {
			if strings.HasPrefix(txt, "dnslink=") {
				txt = string(txt[:8])
			}

			return txt, nil
		}
	}

	return "", fmt.Errorf("invalid TXT record")
}

type fileResolver struct {
	root string
}

func NewFileResolver(uri string) SkynetLinkResolver {
	p := filepath.Clean(strings.TrimPrefix(uri, "file:"))

	return &fileResolver{ root: p }
}

func (r *fileResolver) Resolve(repo, ref string) []string {
	if ref == "" {
		files, err := os.ReadDir(fmt.Sprintf("%s/%s", r.root, repo))
		if err != nil {
			return nil
		}

		var sa []string

		for _, f := range files {
			if f.Type().IsRegular() {
				sa = append(sa, f.Name())
			}
		}

		return sa
	}

	fullPath := fmt.Sprintf("%s/%s/%s", r.root, repo, ref)
	if bz, err := os.ReadFile(fullPath); err == nil {
		return []string{strings.TrimSpace(string(bz))}
	}

	return nil
}

type skynetResolver struct {
	client *skynet.Client
	skynetLink string
}

func NewSkynetResolver(client *skynet.Client, root string) (SkynetLinkResolver,) {
	return &skynetResolver{ client: client, skynetLink: root,}
}

func (r *skynetResolver) Resolve(repo, ref string) []string {
	if ref == "" {
		path := fmt.Sprintf("%s/%s", r.skynetLink, repo)
		links, err := r.client.List(path)
		if err != nil {
			return nil
		}

		var sa []string
		for _, l := range links {
			sa = append(sa, l.Name)
		}

		return sa
	}

	if bz, err := r.getContent(repo, ref); err == nil {
		return []string{strings.TrimSpace(string(bz))}
	}

	return nil
}


func (r *skynetResolver) getContent(repo, ref string) ([]byte, error) {
	resp, err := r.client.Download(fmt.Sprintf("%s/%s/%s", r.skynetLink, repo, ref))
	if err != nil {
		return nil, err
	}

	defer resp.Close()

	return io.ReadAll(resp)
}

type resolver struct {
	resolvers []SkynetLinkResolver
}

func NewResolver(client *skynet.Client, list []string) SkynetLinkResolver {
	var resolvers []SkynetLinkResolver

	for _, l := range list {
		switch {
		case strings.HasPrefix(l , "file:"):
			resolvers = append(resolvers, NewFileResolver(l))
		case strings.HasPrefix(l , "/skynet/"):
			resolvers = append(resolvers, NewSkynetResolver(client, l))
		default:
			resolvers = append(resolvers, NewSkynetResolver(client, l))
		}
	}

	return &resolver{resolvers: resolvers}
}

func (r *resolver) Resolve(repo, ref string) []string {
	var list []string

	for _, res := range r.resolvers {
		if resp := res.Resolve(repo, ref); resp != nil {
			if ref != "" {
				return resp
			}
			list = append(list, resp...)
		}
	}

	sort.Strings(uniq(list))
	return list
}

func uniq(sa []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, s := range sa {
		if _, ok := keys[s]; !ok {
			keys[s] = true
			list = append(list, s)
		}
	}
	return list
}
