package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func main() {
	cmd := &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, args)
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("invalid number of arguments")
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringArrayP("header", "H", nil, "HTTP header to add to the request when proxying")
	flags.String("addr", ":8080", "address to listen on")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	target, err := url.Parse(args[0])
	if err != nil {
		return err
	}

	flags := cmd.Flags()

	headers, err := flags.GetStringArray("header")
	if err != nil {
		return err
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			scheme := "http"
			if pr.Out.URL.Scheme != "" {
				scheme = pr.Out.URL.Scheme
			}
			if pr.Out.Header.Get("X-Forwarded-Proto") != "" {
				scheme = pr.Out.Header.Get("X-Forwarded-Proto")
			}
			if pr.Out.TLS != nil {
				scheme = "https"
			}
			originalURL := fmt.Sprintf("%s://%s%s", scheme, pr.Out.Host, pr.Out.URL)

			pr.SetURL(target)

			newURL := pr.Out.URL.String()

			log.Printf("%s -> %s", originalURL, newURL)

			pr.Out.Host = target.Host
			pr.Out.Header.Set("Host", target.Host)

			for _, header := range headers {
				parts := strings.Split(header, "=")
				key := parts[0]
				value := strings.Join(parts[1:], "=")
				pr.Out.Header.Set(key, value)
			}
		},
	}

	addr, err := flags.GetString("addr")
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:    addr,
		Handler: proxy,
	}

	g := &errgroup.Group{}
	g.Go(server.ListenAndServe)

	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer cancel()

	g.Go(func() error {
		<-ctx.Done()
		return server.Shutdown(ctx)
	})

	if err := g.Wait(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
