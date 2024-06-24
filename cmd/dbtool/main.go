package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"github.com/cybroslabs/hes-1-dbtool/internal/bootstrap"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path"
	"regexp"
	"sort"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Naming
// path: file path to the SQL file
// hash: checksum of the SQL file

var (
	Version = "dev"

	re_FILENAME = regexp.MustCompile(`^[a-z0-9]+[a-z0-9-_]*.sql$`)
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := bootstrap.Logger()
	defer func() { _ = logger.Sync() }()

	logger.Info("Starting clbs-dbtool")

	logger.Info("Loading config...")

	cfg := &config{}
	err := loadConfig(cfg)
	if err != nil {
		logger.Fatal("Error loading config", zap.Error(err))
	}

	logger.Info("Looking for SQL files", zap.String("dir", cfg.dir))

	var sqlFiles []sqlFile

	err = readDir(&sqlFiles, cfg.dir, "")
	if err != nil {
		logger.Fatal("Error reading dir", zap.Error(err))
	}

	sort.Slice(sqlFiles, func(i, j int) bool {
		return sqlFiles[i].path < sqlFiles[j].path
	})

	logger.Info("Found matching SQL files:")
	for _, f := range sqlFiles {
		logger.Info(fmt.Sprintf("  %s\n", f.path))
	}

	logger.Info("Connecting to database...")

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 5*time.Second)
	defer timeoutCancel()

	conn, err := pgx.Connect(timeoutCtx, cfg.databaseURL)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Fatal("Error connecting to database: timeout")
		}

		logger.Fatal("Error connecting to database", zap.Error(err))
	}
	defer func() {
		err = conn.Close(ctx)
		if err != nil {
			logger.Fatal("Error closing connection", zap.Error(err))
		}
	}()

	logger.Info("Ensuring migration table exists...")

	err = ensureMigrationTableExists(*conn)
	if err != nil {
		logger.Fatal("Error ensuring migration table exists", zap.Error(err))
	}

	err = prepareListOfMigrations(*conn, sqlFiles, cfg)
	if err != nil {
		logger.Fatal("Error preparing list of migrations", zap.Error(err))
	}

	logger.Info("Migrations to apply:")
	for _, f := range sqlFiles {
		if !f.process {
			continue
		}
		logger.Infof("  %s\n", f.path)
	}

	logger.Info("Applying migrations...")
	err = applyMigrations(conn, cfg.dir, sqlFiles)
	if err != nil {
		logger.Fatalf("Error applying migrations: %v", err)
	}

	logger.Info("clbs-dbtool finished")
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
	path    string
	hash    string
	process bool
}

// readDir reads the directory recursively and appends all SQL files to the sqlFiles slice
func readDir(sqlFiles *[]sqlFile, rootDir string, subDir string) error {
	currentDir := path.Join(rootDir, subDir)
	entry, err := os.ReadDir(currentDir)
	if err != nil {
		return err
	}

	for _, e := range entry {
		entryPath := path.Join(subDir, e.Name())

		// depth first
		if e.IsDir() {
			err := readDir(sqlFiles, rootDir, entryPath)
			if err != nil {
				return err
			}

			continue
		}

		// is file name invalid? -> continue
		if !isValidFileName(e.Name()) {
			return fmt.Errorf("invalid file name '%s' in folder %s", e.Name(), subDir)
		}

		checksum, err := getFileChecksum(entryPath)
		if err != nil {
			return err
		}

		*sqlFiles = append(*sqlFiles, sqlFile{path: entryPath, hash: checksum,
			process: false,
		})
	}

	return nil
}

// isValidFileName checks if the file name is valid
func isValidFileName(name string) bool {
	return re_FILENAME.MatchString(name)
}

// getFileChecksum returns the sha256 checksum of the file
func getFileChecksum(path string) (string, error) {
	h := sha256.New()

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}

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
	file_path VARCHAR(1024) NOT NULL,
	file_checksum VARCHAR(64) NOT NULL, -- sha256 hash as hex string
	applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  clbs_dbtool_version VARCHAR(10) NOT NULL
)`
	_, err := conn.Exec(context.Background(), createTableSQL)
	if err != nil {
		return err
	}

	return nil
}

func prepareListOfMigrations(conn pgx.Conn, files []sqlFile, cfg *config) error {
	type migration struct {
		path string
		hash string
	}

	//goland:noinspection SqlResolve
	rows, err := conn.Query(context.Background(), "SELECT file_path, file_checksum FROM clbs_dbtool.migrations_v0 ORDER BY id ASC")
	if err != nil {
		return err
	}
	defer rows.Close()

	appliedMigrations := make([]migration, 0)
	for rows.Next() {
		var m migration
		err := rows.Scan(&m.path, &m.hash)
		if err != nil {
			return err
		}
		appliedMigrations = append(appliedMigrations, m)
	}

	appliedMigrationsChan := make(chan migration, len(appliedMigrations))
	defer close(appliedMigrationsChan)

	for _, m := range appliedMigrations {
		appliedMigrationsChan <- m
	}

	toBeApplied := 0
	for idx, f := range files {
		select {
		case m := <-appliedMigrationsChan:
			if m.path != f.path {
				return fmt.Errorf("file %s has been moved since applied, %s", f.path, m.path)
			}
			if m.hash != f.hash {
				if cfg.skipFileValidation {
					continue
				}

				return fmt.Errorf("file %s has changed", f.path)
			}

			continue
		default:
		}

		if toBeApplied == cfg.steps {
			break
		}

		files[idx].process = true
		toBeApplied++
	}

	return nil
}

func applyMigrations(conn *pgx.Conn, rootDir string, files []sqlFile) error {
	for _, f := range files {
		if !f.process {
			continue
		}

		fd, err := os.Open(path.Join(rootDir, f.path))
		if err != nil {
			return err
		}

		sql, err := readText(fd)
		if err != nil {
			return err
		}

		_, err = conn.Exec(context.Background(), sql)
		if err != nil {
			return err
		}

		//goland:noinspection SqlResolve
		_, err = conn.Exec(context.Background(), "INSERT INTO clbs_dbtool.migrations_v0 (file_path, file_checksum, clbs_dbtool_version) VALUES ($1, $2, $3)", f.path, f.hash, Version)
		if err != nil {
			return err
		}
	}

	return nil
}

// readText reads the text from the reader and returns it as a string
// it also handles the BOM (Byte Order Mark) at the beginning of the file
func readText(reader io.Reader) (string, error) {
	var transformer = unicode.BOMOverride(encoding.Nop.NewDecoder())
	tmp := &bytes.Buffer{}
	_, err := tmp.ReadFrom(transform.NewReader(reader, transformer))
	if err != nil {
		return "", err
	}
	return tmp.String(), nil
}
