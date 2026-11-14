package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"skillshare/internal/skillserve"
)

func serveMCPHTTP(handler http.Handler, addr, certFile, keyFile string, allowInsecure bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	httpSrv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		httpSrv.Close()
	}()
	scheme := "https"
	if certFile == "" {
		scheme = "http"
	}
	if allowInsecure && certFile == "" && !isLoopbackAddr(addr) {
		fmt.Fprintln(os.Stderr, "warning: --http-allow-insecure sends the bearer token over plain HTTP; use only on a trusted private network")
	}
	fmt.Fprintf(os.Stderr, "Serving skills over MCP at %s://%s/\n", scheme, ln.Addr())
	fmt.Fprintf(os.Stderr, "Executor skill index at %s://%s%sindex.json\n", scheme, ln.Addr(), skillserve.IndexPath)

	if certFile == "" {
		err = httpSrv.Serve(ln)
	} else {
		err = httpSrv.ServeTLS(ln, certFile, keyFile)
	}
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// isLoopbackAddr reports whether a listen address only accepts local
// connections. An empty host listens on every interface.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
