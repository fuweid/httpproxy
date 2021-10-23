package main

import (
	"log"
	"net/http"
	"time"

	"github.com/fuweid/httpproxy/pkg/proxy"
)

func main() {
	srv := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(proxy.NewProxyServer(
			proxy.LimitRule{
				LimitedBytesPerSec: 1024 * 1024 * 8, /* 8 MB */
				RetryAfter:         time.Second * 5, /* 5 seconds */
			},
		)),
	}
	log.Fatal(srv.ListenAndServe())
}
