package main

import (
	"fmt"
	"os"

	"github.com/edgeflare/pgo/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:   "pgo",
	Short: "PGO is a PostgreSQL CDC tool",
	Long:  `PGO is a PostgreSQL Change Data Capture (CDC) tool that replicates data changes to various destinations.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/pgo.yaml)")
	rootCmd.PersistentFlags().String("postgres.logrepl_conn_string", "", "PostgreSQL logical replication connection string")
	rootCmd.PersistentFlags().String("postgres.tables", "", "Comma-separated list of tables to replicate")

	viper.BindPFlag("postgres.logrepl_conn_string", rootCmd.PersistentFlags().Lookup("postgres.logrepl_conn_string"))
	viper.BindPFlag("postgres.tables", rootCmd.PersistentFlags().Lookup("postgres.tables"))

	// Add the pipeline subcommand
	rootCmd.AddCommand(pipelineCmd)
}

func initConfig() {
	var err error
	cfg, err = config.LoadConfig(cfgFile)
	if err != nil {
		fmt.Println("Error loading config:", err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}