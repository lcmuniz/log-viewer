// Command log-viewer serves a small web UI to follow log files in a folder.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/lcmuniz/log-viewer/internal/viewer"
)

func main() {
	var (
		refresh   = flag.Int("r", 0, "refresh interval in seconds (default 5, saved in log-viewer.json)")
		folder    = flag.String("l", ".", "log folder path")
		theme     = flag.String("t", "", `theme: "light" or "dark"`)
		addr      = flag.String("addr", "127.0.0.1:3000", "listen address; use 0.0.0.0:3000 to expose on the network (no authentication!)")
		maxBytes  = flag.Int64("max-bytes", viewer.DefaultMaxBytes, "maximum bytes shown from the end of each file")
		noBrowser = flag.Bool("no-browser", false, "do not open the browser automatically")
	)
	flag.Parse()

	cfg, written, err := viewer.LoadConfig(*folder, *refresh, *theme)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if written {
		fmt.Printf("Saved %s in %s\n", viewer.ConfigFileName, cfg.LogFolder)
	}

	loopback := viewer.IsLoopback(*addr)
	if !loopback {
		log.Printf("WARNING: listening on %s without authentication; anyone who can reach this address can read your logs", *addr)
	}

	srv := &http.Server{
		Addr: *addr,
		Handler: viewer.New(cfg, viewer.Options{
			MaxBytes:  *maxBytes,
			CheckHost: loopback,
		}).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	url := "http://" + *addr + "/"
	fmt.Printf("Log Viewer running at %s\n", url)
	fmt.Printf("Watching folder: %s (refresh %ds)\n", cfg.LogFolder, cfg.RefreshInterval)

	if !*noBrowser && loopback {
		go func() {
			time.Sleep(500 * time.Millisecond)
			if err := openBrowser(url); err != nil {
				fmt.Printf("Could not open the browser (%v); open %s manually\n", err, url)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
	return cmd.Start()
}
