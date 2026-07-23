package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hellogxp/stateseal/internal/ui"
	"github.com/spf13/cobra"
)

func uiCmd() *cobra.Command {
	var address string
	var noOpen bool
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Open the local StateSeal runs console",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateLoopbackAddress(address); err != nil {
				return codedError{10, err}
			}
			listener, err := net.Listen("tcp", address)
			if err != nil {
				return codedError{11, fmt.Errorf("start StateSeal UI: %w", err)}
			}
			defer listener.Close()
			console, err := ui.NewServer()
			if err != nil {
				return codedError{11, err}
			}
			server := &http.Server{
				Handler:           console.Handler(),
				ReadHeaderTimeout: 5 * time.Second,
				IdleTimeout:       75 * time.Second,
			}
			serverErrors := make(chan error, 1)
			go func() {
				if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
					serverErrors <- err
				}
			}()
			url := "http://" + listener.Addr().String()
			fmt.Fprintf(cmd.OutOrStdout(), "StateSeal Runs: %s\nTrusted local state · read-only · Ctrl+C to stop\n", url)
			if !noOpen {
				if err := openBrowser(url); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Could not open a browser automatically: %v\n", err)
				}
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			select {
			case <-ctx.Done():
			case err := <-serverErrors:
				return codedError{11, err}
			}
			// The only long-lived requests are read-only SSE streams. Closing
			// them immediately makes Ctrl+C deterministic and cannot interrupt a
			// state-changing operation.
			if err := server.Close(); err != nil {
				return codedError{11, err}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&address, "address", "127.0.0.1:0", "loopback address to listen on")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "print the URL without opening a browser")
	return cmd
}

func validateLoopbackAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid UI address %q: %w", address, err)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("StateSeal UI only listens on a loopback address, got %q", host)
	}
	return nil
}

func openBrowser(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	return command.Start()
}
