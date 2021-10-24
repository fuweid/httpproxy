package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/fuweid/httpproxy/pkg/proxy"
	"github.com/sirupsen/logrus"
)

var (
	flagHttpPort           = 8080
	flagLimitedBytesPerSec = 1024 * 1024 * 8
	flagRetryAfterSec      = 5

	minLimitedBytes = 1 << 12
)

func main() {
	flag.IntVar(&flagHttpPort, "port", flagHttpPort, "http proxy port")
	flag.IntVar(&flagLimitedBytesPerSec, "rate", flagLimitedBytesPerSec, "limited bytes per second during transport (in bytes)")
	flag.IntVar(&flagRetryAfterSec, "retry-after", flagRetryAfterSec, "retry after when hit the rate limit (in second)")
	flag.Parse()

	if flagLimitedBytesPerSec < minLimitedBytes {
		logrus.Warnf("the rate is invalid(< %v), rate limited will be disabled", minLimitedBytes)
	}

	srv := &http.Server{
		Addr: fmt.Sprintf(":%v", flagHttpPort),
		Handler: http.HandlerFunc(proxy.NewProxyServer(
			proxy.LimitRule{
				LimitedBytesPerSec: flagLimitedBytesPerSec,
				RetryAfter:         time.Second * time.Duration(flagRetryAfterSec),
			},
		)),
	}
	log.Fatal(srv.ListenAndServe())
}
