package server

import (
	"fmt"
	"net"
	"net/http"

	"github.com/jay-dee7/parachute/config"
	"github.com/jay-dee7/parachute/server/registry"
	"github.com/rs/zerolog"
)

type Server struct {
	c  *config.RegistryConfig
	ln net.Listener
	l  zerolog.Logger
}

type InfoResponse struct {
	Info        string   `json:"what"`
	Project     string   `json:"project"`
	Portal      string   `json:"portal"`
	Handles     []string `json:"handles"`
	Problematic []string `json:"problematic"`
}

func NewServer(logger zerolog.Logger, c *config.RegistryConfig) *Server {
	return &Server{c: c, ln: nil, l: logger}
}

func (s *Server) Start() error {
	if s.ln != nil {
		return nil
	}

	http.HandleFunc("/health", func(rw http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(rw, "OK")
	})

	http.Handle("/", registry.New(s.l, s.c))

	ln, err := net.Listen("tcp", s.c.Address())
	if err != nil {
		return err
	}

	s.ln = ln

	if s.c.TLSCertPath != "" && s.c.TLSKeyPath != "" {
		return http.ServeTLS(s.ln, nil, s.c.TLSCertPath, s.c.TLSKeyPath)
	}

	s.Debugf("server started on", s.c.Address())

	return http.Serve(s.ln, nil)
}

func (s *Server) Stop() error {
	if s.ln != nil {
		return s.ln.Close()
	}

	return nil
}

func (s *Server) Debugf(str string, args ...interface{}) {
	if s.c.Debug {
		e := s.l.Debug()
		e.Msgf(str, args...)
	}
}
