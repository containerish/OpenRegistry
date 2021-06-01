package server

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/jay-dee7/parachute/server/registry"
)

type Server struct {
	debug               bool
	listener            net.Listener
	host                string
	skynetHost          string
	SkynetPortalURL     string
	skynetLinkResolvers []string
	skynetStorePath     string
	tlsCertPath         string
	tlsKeyPath          string
}

type Config struct {
	Debug               bool
	Port                uint
	SkynetHost          string
	SkynetPortalURL     string
	SkynetLinkResolvers []string
	SkynetStorePath     string
	TLSCertPath         string
	TLSKeyPath          string
}

type InfoResponse struct {
	Info        string   `json:"what"`
	Project     string   `json:"project"`
	Portal      string   `json:"portal"`
	Handles     []string `json:"handles"`
	Problematic []string `json:"problematic"`
}


var projectURL = "https://github.com/jay-dee7/parachute"

func NewServer(config *Config) *Server {
	if config == nil {
		config = &Config{}
	}

	if config.Port == 0 {
		config.Port = 5000
	}

	return &Server{
		debug:               config.Debug,
		listener:            nil,
		host:                fmt.Sprintf("100.87.37.43:%d", config.Port),
		skynetHost:          config.SkynetHost,
		SkynetPortalURL:     config.SkynetPortalURL,
		skynetLinkResolvers: config.SkynetLinkResolvers,
		skynetStorePath:      config.SkynetStorePath,
		tlsCertPath:         "",
		tlsKeyPath:          "",
	}
}

func (s *Server) registryConfig() *registry.Config {
	return &registry.Config{
		SkynetHost: s.skynetHost,
		SkynetPortal: s.SkynetPortalURL,
		SkynetResolvers: s.skynetLinkResolvers,
		SkynetStorePath: s.skynetStorePath,
	}
}

func (s *Server) Start() error {
	if s.listener != nil {
		return nil
	}

	http.HandleFunc("/health", func(rw http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(rw, "OK")
	})

	http.Handle("/", registry.New(s.registryConfig()))

	ln, err := net.Listen("tcp", s.host)
	if err != nil {
		return err
	}

	s.listener = ln

	if s.tlsKeyPath != "" && s.tlsCertPath != "" {
		return http.ServeTLS(s.listener, nil, s.tlsCertPath, s.tlsKeyPath)
	}

	fmt.Printf("started server on: %s", s.host)

	return http.Serve(s.listener, nil)
}

func (s *Server) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}

	return nil
}

func (s *Server) Debugf(str string, args ...interface{}) {
	if s.debug {
		log.Printf(str, args...)
	}
}

