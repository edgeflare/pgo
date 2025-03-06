package pgo

import (
	"cmp"
	"log"
	"os"

	"github.com/edgeflare/pgo/pkg/rest"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var restCmd = &cobra.Command{
	Use:   "rest",
	Short: "Start the REST API server",
	Long:  `Starts a REST API server that provides access to PostgreSQL data through HTTP endpoints`,
	Run: func(cmd *cobra.Command, args []string) {
		connString := viper.GetString("rest.pg_conn_string")
		listenAddr := viper.GetString("rest.listen_addr")

		connString = cmp.Or(connString, os.Getenv("PGO_REST_PG_CONN_STRING"))
		if connString == "" {
			log.Fatal("PostgreSQL connection string is required. Set it via --rest.pg_conn_string flag or in config file.")
		}

		server, err := rest.NewServer(connString, "")
		if err != nil {
			log.Fatalf("Failed to create server: %v", err)
		}
		defer server.Shutdown()

		log.Printf("Starting REST server on %s", listenAddr)
		log.Fatal(server.Start(listenAddr))
	},
}

func init() {
	restCmd.Flags().StringP("rest.pg_conn_string", "c", "", "PostgreSQL connection string")
	restCmd.Flags().StringP("rest.listen_addr", "l", ":8080", "Address for the REST server to listen on")

	viper.BindPFlag("rest.pg_conn_string", restCmd.Flags().Lookup("rest.pg_conn_string"))
	viper.BindPFlag("rest.listen_addr", restCmd.Flags().Lookup("rest.listen_addr"))

	rootCmd.AddCommand(restCmd)
}
