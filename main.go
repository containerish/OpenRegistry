package main

import (
	"log"

	"github.com/jay-dee7/parachute/server"
)
func main() {
	config := &server.Config{
		Debug:               true,
		Port:                5000,
		SkynetHost:          "",
		SkynetPortalURL:     "https://siasky.net",
		SkynetLinkResolvers: []string{"file:", "/Users/jasdeep/.parachute/skylinks"},
		SkynetStorePath:     "/Users/jasdeep/.parachute/skyklinks",
		TLSCertPath:         "",
		TLSKeyPath:          "",
	}

	srv := server.NewServer(config)
	defer srv.Stop()

	log.Fatalln(srv.Start())
}
