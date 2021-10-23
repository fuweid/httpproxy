package main

import (
	"log"
	"net/http"

	"github.com/fuweid/httpproxy/pkg/proxy"
)

func main() {
	srv := &http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(proxy.NewProxyServer()),
	}
	log.Fatal(srv.ListenAndServe())
}
