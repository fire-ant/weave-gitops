package run

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-logr/zapr"
	"github.com/mattn/go-isatty"
	"github.com/pkg/browser"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/weaveworks/weave-gitops/pkg/server"
	"go.uber.org/zap"
)

var (
	port string
	path string
)

var Cmd = &cobra.Command{
	Use:   "run",
	Short: "Runs wego ui",
	RunE:  runCmd,
}

func runCmd(cmd *cobra.Command, args []string) error {
	var log = logrus.New()

	mux := http.NewServeMux()

	mux.Handle("/health/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("ok"))

		if err != nil {
			log.Errorf("error writing health check: %s", err)
		}
	}))

	assetFS := getAssets()
	assetHandler := http.FileServer(http.FS(assetFS))
	redirector := createRedirector(assetFS, log)

	cfg, err := server.DefaultConfig()
	if err != nil {
		return fmt.Errorf("could not create http client: %w", err)
	}

	logr := zapr.NewLogger(zap.NewNop())

	cfg.Logger = logr

	appsHandler, err := server.NewApplicationsHandler(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("could not create applications handler: %w", err)
	}

	mux.Handle("/v1/", appsHandler)

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Assume anything with a file extension in the name is a static asset.
		extension := filepath.Ext(req.URL.Path)
		// We use the golang http.FileServer for static file requests.
		// This will return a 404 on normal page requests, ie /some-page.
		// Redirect all non-file requests to index.html, where the JS routing will take over.
		if extension == "" {
			redirector(w, req)
			return
		}
		assetHandler.ServeHTTP(w, req)
	}))

	addr := "0.0.0.0:" + port
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Infof("Serving on port %s", port)

		if err := srv.ListenAndServe(); err != nil {
			log.Error(err, "server exited")
			os.Exit(1)
		}
	}()

	if isatty.IsTerminal(os.Stdout.Fd()) {
		url := fmt.Sprintf("http://%s/%s", addr, path)

		log.Printf("Openning browser at %s", url)

		if err := browser.OpenURL(url); err != nil {
			return fmt.Errorf("failed to open the browser: %w", err)
		}
	}

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer func() {
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("Server Shutdown Failed: %w", err)
	}

	return nil
}

//go:embed dist/*
var static embed.FS

func getAssets() fs.FS {
	f, err := fs.Sub(static, "dist")

	if err != nil {
		panic(err)
	}

	return f
}

// A redirector ensures that index.html always gets served.
// The JS router will take care of actual navigation once the index.html page lands.
func createRedirector(fsys fs.FS, log logrus.FieldLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		indexPage, err := fsys.Open("index.html")

		if err != nil {
			log.Error(err, "could not open index.html page")
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		stat, err := indexPage.Stat()
		if err != nil {
			log.Error(err, "could not get index.html stat")
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		bt := make([]byte, stat.Size())
		_, err = indexPage.Read(bt)

		if err != nil {
			log.Error(err, "could not read index.html")
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		_, err = w.Write(bt)

		if err != nil {
			log.Error(err, "error writing index.html")
			w.WriteHeader(http.StatusInternalServerError)

			return
		}
	}
}

func init() {
	Cmd.Flags().StringVar(&port, "port", "9001", "UI port")
	Cmd.Flags().StringVar(&path, "path", "", "Path url")
}
