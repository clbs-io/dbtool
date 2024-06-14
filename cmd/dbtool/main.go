package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
)

// Naming
// path: file path to the SQL file
// hash: checksum of the SQL file

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("Starting clbs-dbtool")

	logger.Println("Loading config...")

	cfg := &config{}
	err := loadConfig(cfg)
	if err != nil {
		logger.Fatalf("Error loading config: %v", err)
	}

	logger.Printf("Looking for SQL files in %s\n", cfg.dir)

	var sqlFiles []sqlFile

	err = readDir(&sqlFiles, cfg.dir)
	if err != nil {
		logger.Fatalf("Error reading dir: %v", err)
	}

	sort.Slice(sqlFiles, func(i, j int) bool {
		return sqlFiles[i].path < sqlFiles[j].path
	})

	logger.Println("Found matching SQL files:")
	for _, f := range sqlFiles {
		logger.Printf("  %s\n", f.path)
	}

	logger.Println("Connecting to database...")

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 5*time.Second)
	defer timeoutCancel()

	conn, err := pgx.Connect(timeoutCtx, cfg.databaseURL)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Fatalf("Error connecting to database: timeout")
		}

		logger.Fatalf("Error connecting to database: %v", err)
	}
	defer func() {
		err = conn.Close(ctx)
		if err != nil {
			logger.Fatalf("Error closing connection: %v", err)
		}
	}()

	logger.Println("Ensuring migration table exists...")

	err = ensureMigrationTableExists(*conn)
	if err != nil {
		logger.Fatalf("Error ensuring migration table exists: %v", err)
	}

	toApply, err := prepareListOfMigrations(*conn, sqlFiles, cfg.dir, cfg)
	if err != nil {
		logger.Fatalf("Error preparing list of migrations: %v", err)
	}

	logger.Println("Migrations to apply:")
	for _, f := range toApply {
		logger.Printf("  %s\n", f.path)
	}

	logger.Println("Applying migrations...")
	err = applyMigrations(conn, toApply)
	if err != nil {
		logger.Fatalf("Error applying migrations: %v", err)
	}

	logger.Println("clbs-dbtool finished")
}

type config struct {
	dir                string
	databaseURL        string
	steps              int
	skipFileValidation bool
}

var defaultSteps = -1

func loadConfig(cfg *config) error {
	flag.StringVar(&cfg.dir, "migrations-dir", "", "Root directory where to look for SQL files")
	flag.StringVar(&cfg.databaseURL, "database-url", "", "Database URL to connect to")
	flag.IntVar(&cfg.steps, "steps", defaultSteps, "Number of steps to apply")
	flag.BoolVar(&cfg.skipFileValidation, "skip-file-validation", false, "Skip file validation")

	flag.Parse()

	if cfg.dir == "" {
		return fmt.Errorf("migrations-dir is required")
	}

	fileInfo, err := os.Stat(cfg.dir)
	if err != nil {
		return fmt.Errorf("dir %s does not exist", cfg.dir)
	}

	if !fileInfo.IsDir() {
		return fmt.Errorf("dir %s is not a directory", cfg.dir)
	}

	if cfg.databaseURL == "" {
		return fmt.Errorf("database-url is required")
	}

	// use url.ParseRequestURI() to validate the URL, not url.Parse(), since almost anything is valid for url.Parse()
	parsedURL, err := url.ParseRequestURI(cfg.databaseURL)
	if err != nil {
		return fmt.Errorf("database-url is not a valid URL")
	}

	if parsedURL.Scheme != "postgres" {
		return fmt.Errorf("database-url scheme must be 'postgres'")
	}

	return nil
}

type sqlFile struct {
	path string
	hash string
}

func readDir(sqlFiles *[]sqlFile, dir string) error {
	entry, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, e := range entry {
		// depth first
		if e.IsDir() {
			err = readDir(sqlFiles, dir+"/"+e.Name())
			if err != nil {
				return err
			}

			continue
		}

		// is file name invalid? -> continue
		if !isValidFileName(e.Name()) {
			continue
		}

		checksum, err := getFileChecksum(dir + "/" + e.Name())
		if err != nil {
			return err
		}

		*sqlFiles = append(*sqlFiles, sqlFile{path: dir + "/" + e.Name(), hash: checksum})
	}

	return nil
}

func isValidFileName(name string) bool {
	pattern := "^[a-z0-9]+[a-z0-9-_]+.sql$"

	matched, err := regexp.MatchString(pattern, name)
	if err != nil {
		return false
	}

	return matched
}

func getFileChecksum(path string) (string, error) {
	h := sha256.New()

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(h, f)
	checksum := hex.EncodeToString(h.Sum(nil))

	err = f.Close()
	if err != nil {
		return "", err
	}

	return checksum, nil
}

func ensureMigrationTableExists(conn pgx.Conn) error {
	const createTableSQL = `
CREATE SCHEMA IF NOT EXISTS clbs_dbtool;
CREATE TABLE IF NOT EXISTS clbs_dbtool.migrations_v0 (
	id BIGSERIAL PRIMARY KEY,
	file VARCHAR(500) NOT NULL,
	file_checksum VARCHAR(64) NOT NULL, -- sha256 hash as hex string
	root_dir VARCHAR(500) NOT NULL,
	applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  clbs_dbtool_version VARCHAR(10) NOT NULL
)`
	_, err := conn.Exec(context.Background(), createTableSQL)
	if err != nil {
		return err
	}

	return nil
}

func prepareListOfMigrations(conn pgx.Conn, files []sqlFile, rootDir string, cfg *config) ([]sqlFile, error) {
	type migration struct {
		path string
		hash string
	}

	//goland:noinspection SqlResolve
	rows, err := conn.Query(context.Background(), "SELECT file, file_checksum, root_dir FROM clbs_dbtool.migrations_v0 ORDER BY file DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	appliedMigrations := make([]migration, 0)
	for rows.Next() {
		var m migration
		err := rows.Scan(&m.path, &m.hash)
		if err != nil {
			return nil, err
		}

		//if m.rootDir != rootDir {
		//	return nil, fmt.Errorf("root dir mismatch for file %s: %s != %s", m.file, m.rootDir, rootDir)
		//}

		appliedMigrations = append(appliedMigrations, m)
	}

	appliedMigrationsChan := make(chan migration, len(appliedMigrations))
	defer close(appliedMigrationsChan)

	for _, m := range appliedMigrations {
		appliedMigrationsChan <- m
	}

	var toApply []sqlFile

	toBeApplied := 0
	for idx, f := range files {
		select {
		case m := <-appliedMigrationsChan:
			if m.path != f.path {
				return nil, fmt.Errorf("file %s has been moved since applied, %s", f.path, m.path)
			}
			if m.hash != f.hash {
				if cfg.skipFileValidation {
					continue
				}

				return nil, fmt.Errorf("file %s has changed", f.path)
			}

			continue
		default:
		}

		if toBeApplied == cfg.steps {
			break
		}

		toApply = append(toApply, files[idx])
		toBeApplied++
	}

	return toApply, nil
}

func applyMigrations(conn *pgx.Conn, files []sqlFile) error {
	for _, f := range files {
		sql, err := os.ReadFile(f.path)
		if err != nil {
			return err
		}

		_, err = conn.Exec(context.TODO(), string(sql))
		if err != nil {
			return err
		}

		//goland:noinspection SqlResolve
		_, err = conn.Exec(context.TODO(), "INSERT INTO clbs_dbtool.migrations_v0 (file, file_checksum, clbs_dbtool_version, root_dir) VALUES ($1, $2, $3, $4)", f.path, f.hash, "v0", "--todo:@lcapka")
		if err != nil {
			return err
		}
	}

	return nil
}
