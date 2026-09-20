package cmd

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"mini-me/internal/embed"
	"mini-me/internal/logger"
	"mini-me/internal/server"
	"mini-me/internal/store"
)

var (
	flagServeHost string
	flagServePort int
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP REST API server bound strictly to 127.0.0.1",
	Long:  `serve starts the HTTP REST API server on localhost only. Authenticated via bearer token stored in a 0600 permissions auth_token file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer stop()

		dbPath, err := GetDBPath()
		if err != nil {
			return fmt.Errorf("failed resolving db path: %w", err)
		}

		st, err := store.Open(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("failed opening database: %w", err)
		}
		defer st.Close()

		token, err := server.GetOrCreateAuthToken()
		if err != nil {
			return fmt.Errorf("failed getting auth token: %w", err)
		}

		embedClient := embed.NewClient()
		srv := server.NewServer(st, embedClient, flagServeHost, flagServePort, token)

		logger.Info("starting mini-me HTTP server", "address", srv.Addr())
		fmt.Printf("mini-me HTTP server listening at http://%s\n", srv.Addr())
		fmt.Printf("Bearer token loaded from config. Header: 'Authorization: Bearer %s'\n", token)

		errChan := make(chan error, 1)
		go func() {
			errChan <- srv.Start(ctx)
		}()

		select {
		case <-ctx.Done():
			fmt.Println("\nShutting down HTTP server...")
			return srv.Close()
		case err := <-errChan:
			return err
		}
	},
}

func init() {
	serveCmd.Flags().StringVar(&flagServeHost, "host", "127.0.0.1", "bind host (default 127.0.0.1)")
	serveCmd.Flags().IntVar(&flagServePort, "port", 8080, "bind port (default 8080)")
	RootCmd.AddCommand(serveCmd)
}
