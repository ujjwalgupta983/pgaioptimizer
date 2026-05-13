package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	version = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "pgaioptimizer",
	Short: "PostgreSQL AI Performance Analyzer",
	Long: `A tool that connects to PostgreSQL, collects deep metrics from system catalogs, 
analyzes them against best practices, and delivers prioritized, actionable recommendations.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pgaioptimizer.yaml)")
	rootCmd.PersistentFlags().StringP("host", "H", "localhost", "database host")
	rootCmd.PersistentFlags().IntP("port", "p", 5432, "database port")
	rootCmd.PersistentFlags().StringP("db", "d", "postgres", "database name")
	rootCmd.PersistentFlags().StringP("user", "U", "postgres", "database user")
	rootCmd.PersistentFlags().String("password", "", "database password")
	rootCmd.PersistentFlags().String("dsn", "", "PostgreSQL connection string (overrides individual flags)")

	viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("db", rootCmd.PersistentFlags().Lookup("db"))
	viper.BindPFlag("user", rootCmd.PersistentFlags().Lookup("user"))
	viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
	viper.BindPFlag("dsn", rootCmd.PersistentFlags().Lookup("dsn"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".pgaioptimizer")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
