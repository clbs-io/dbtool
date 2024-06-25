package dbtool

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/cybroslabs/hes-1-dbtool/internal/config"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"time"
)

// Naming
// filePath: file filePath to the SQL file
// hash: checksum of the SQL file

var (
	Version = "dev"

	reFilename = regexp.MustCompile(`^[a-z0-9]+[a-z0-9-_]*.sql$`)
)

func Run(ctx context.Context, logger *zap.Logger, cfg *config.Config) {
	logger.Info("Looking for SQL files", zap.String("dir", cfg.Dir()))

	var sqlFiles []sqlFile

	err := readDir(&sqlFiles, cfg.Dir(), "")
	if err != nil {
		logger.Fatal("Error reading dir", zap.Error(err))
	}

	sort.Slice(sqlFiles, func(i, j int) bool {
		return sqlFiles[i].path < sqlFiles[j].path
	})

	logger.Debug("Found matching SQL files:")
	for _, f := range sqlFiles {
		logger.Debug(fmt.Sprintf("- %s", f.path))
	}

	logger.Info("Connecting to database...")

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 5*time.Second)
	defer timeoutCancel()

	conn, err := pgx.Connect(timeoutCtx, cfg.DatabaseURL())
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

	logger.Info("Pinging the database...")
	pingErr := conn.Ping(ctx)
	if pingErr != nil {
		logger.Fatal("Could not ping the database", zap.Error(pingErr))
	}

	logger.Info("Ensuring migration table exists...")

	err = ensureMigrationTableExists(*conn)
	if err != nil {
		logger.Fatal("Error ensuring migration table exists", zap.Error(err))
	}

	err = prepareListOfMigrations(*conn, sqlFiles, cfg)
	if err != nil {
		logger.Fatal("Error preparing list of migrations", zap.Error(err))
	}

	logger.Debug("Migrations to apply:")
	for _, f := range sqlFiles {
		if !f.apply {
			continue
		}
		logger.Debug(fmt.Sprintf("- %s", f.path))
	}

	applyErr := applyMigrations(conn, cfg.Dir(), sqlFiles, logger)
	if applyErr != nil {
		logger.Fatal("Error applying migrations", zap.Error(applyErr))
	}

	logger.Info("clbs-dbtool finished")
}

type sqlFile struct {
	path  string
	hash  string
	apply bool
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
			continue
		}

		fileHash, err := getFileHash(path.Join(rootDir, entryPath))
		if err != nil {
			return err
		}

		*sqlFiles = append(*sqlFiles, sqlFile{path: entryPath, hash: fileHash,
			apply: false,
		})
	}

	return nil
}

// isValidFileName checks if the file name is valid
func isValidFileName(name string) bool {
	return reFilename.MatchString(name)
}

// getFileHash returns the sha256 checksum of the file
func getFileHash(path string) (string, error) {
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
	file_hash VARCHAR(64) NOT NULL, -- sha256 hash as hex string
	applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  clbs_dbtool_version VARCHAR(10) NOT NULL
)`
	_, err := conn.Exec(context.Background(), createTableSQL)
	if err != nil {
		return err
	}

	return nil
}

func prepareListOfMigrations(conn pgx.Conn, files []sqlFile, cfg *config.Config) error {
	type migration struct {
		filePath string
		fileHash string
	}

	//goland:noinspection SqlResolve
	rows, err := conn.Query(context.Background(), "SELECT file_path, file_hash FROM clbs_dbtool.migrations_v0 ORDER BY id ASC")
	if err != nil {
		return err
	}
	defer rows.Close()

	appliedMigrations := make([]migration, 0)

	for rows.Next() {
		var m migration
		scanErr := rows.Scan(&m.filePath, &m.fileHash)
		if scanErr != nil {
			return scanErr
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
			if m.filePath != f.path {
				return fmt.Errorf("file %s has been moved since applied, %s", f.path, m.filePath)
			}

			if m.fileHash != f.hash {
				if cfg.SkipFileValidation() {
					continue
				}

				return fmt.Errorf("file %s has changed", f.path)
			}

			// if migration has already been applied, continue
			continue
		default:
		}

		if toBeApplied == cfg.Steps() {
			break
		}

		files[idx].apply = true
		toBeApplied++
	}

	return nil
}

func applyMigrations(conn *pgx.Conn, rootDir string, files []sqlFile) error {
	for _, f := range files {
		if !f.apply {
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
		_, err = conn.Exec(context.Background(), "INSERT INTO clbs_dbtool.migrations_v0 (file_path, file_hash, clbs_dbtool_version) VALUES ($1, $2, $3)", f.path, f.hash, Version)
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
