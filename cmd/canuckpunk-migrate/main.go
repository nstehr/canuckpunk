// Command canuckpunk-migrate applies the schema to the SQLite database.
package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/nstehr/canuckpunk/data/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var dbPath string

	flag.StringVar(&dbPath, "db", "", "path to the SQLite database (or set CANUCKPUNK_DB)")
	flag.Parse()

	if dbPath == "" {
		dbPath = os.Getenv("CANUCKPUNK_DB")
	}

	if dbPath == "" {
		dbPath = "canuckpunk.db"
	}

	args := flag.Args()
	if len(args) < 1 {
		usage()

		return errors.New("no command specified")
	}

	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("error closing database: %v", err)
		}
	}()

	if err := database.Ping(); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	goose.SetBaseFS(migrations.Embed)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	if err := runCommand(database, args[0]); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}

func runCommand(database *sql.DB, command string) error {
	switch command {
	case "up":
		return goose.Up(database, ".")
	case "up-by-one":
		return goose.UpByOne(database, ".")
	case "down":
		return goose.Down(database, ".")
	case "redo":
		return goose.Redo(database, ".")
	case "reset":
		return goose.Reset(database, ".")
	case "status":
		return goose.Status(database, ".")
	case "version":
		return goose.Version(database, ".")
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func usage() {
	fmt.Println("Usage: canuckpunk-migrate [OPTIONS] COMMAND")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  up                   Migrate the DB to the most recent version available")
	fmt.Println("  up-by-one            Migrate the DB up by 1")
	fmt.Println("  down                 Roll back the version by 1")
	fmt.Println("  redo                 Re-run the latest migration")
	fmt.Println("  reset                Roll back all migrations")
	fmt.Println("  status               Dump the migration status for the current DB")
	fmt.Println("  version              Print the current version of the database")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -db string           Path to the SQLite database")
}
