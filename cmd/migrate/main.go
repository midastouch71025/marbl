package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	postgresmig "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"

	dbmigrations "marbl/db/migrations"
	"marbl/internal/config"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: migrate <up|down|down-all|version>")
		os.Exit(2)
	}

	cfg, err := config.LoadMigrate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(2)
	}

	sqlDB, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	driver, err := postgresmig.WithInstance(sqlDB, &postgresmig.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate driver: %v\n", err)
		os.Exit(1)
	}

	src, err := iofs.New(dbmigrations.FS, ".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "embed source: %v\n", err)
		os.Exit(1)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
	defer func() { _, _ = m.Close() }()

	switch args[0] {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			fmt.Fprintf(os.Stderr, "up: %v\n", err)
			os.Exit(1)
		}
	case "down":
		// One migration step only (e.g. 000002 -> 000001). Migrate.Down() rolls back *all* migrations.
		if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			fmt.Fprintf(os.Stderr, "down: %v\n", err)
			os.Exit(1)
		}
	case "down-all":
		if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			fmt.Fprintf(os.Stderr, "down-all: %v\n", err)
			os.Exit(1)
		}
	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			if errors.Is(err, migrate.ErrNilVersion) {
				fmt.Println("version: nil (no migrations applied)")
				return
			}
			fmt.Fprintf(os.Stderr, "version: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		os.Exit(2)
	}
}
