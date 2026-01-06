// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package dbtool

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/clbs-io/dbtool/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Naming
// filePath: file filePath to the SQL file
// hash: checksum of the SQL file

var (
	reFilename = regexp.MustCompile(`^[a-z0-9]+[a-z0-9-_]*.sql$`)
)

type fileType int

const (
	fileTypeUnknown  fileType = iota
	fileTypeSql      fileType = iota
	fileTypeSnapshot fileType = iota
)

func Run(ctx context.Context, logger *zap.Logger, cfg *config.Config) {
	logger.Info("Looking for SQL files", zap.String("dir", cfg.Dir()))

	var sqlFiles []sqlFile

	err := readDir(&sqlFiles, cfg.Dir(), "")
	if err != nil {
		logger.Fatal("Error reading dir", zap.Error(err))
	}

	prepareFiles(sqlFiles)

	logger.Debug("Found matching SQL files:")
	for _, f := range sqlFiles {
		logger.Debug(fmt.Sprintf("- %s", f.path))
	}

	logger.Info(fmt.Sprintf("Connecting to database %s...", cfg.Host()))

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, time.Duration(cfg.ConnectionTimeout())*time.Second)
	defer timeoutCancel()

	// Parse connection string using pgxpool that has more options although we won't use the pool
	conn_config, err := pgxpool.ParseConfig(cfg.ConnectionString())
	if err != nil {
		logger.Fatal("Error parsing connection string", zap.Error(err))
	}

	conn, err := pgx.ConnectConfig(timeoutCtx, conn_config.ConnConfig)
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

	// Detect which migrations need to be applied
	if initialState, err := isInitialState(*conn, cfg); err != nil {
		logger.Fatal("Error checking initial state", zap.Error(err))
	} else if initialState {
		if detect, dir := getLastSnapshot(&sqlFiles); detect {
			logger.Info("The last snapshot detected, skipping migrations before folder " + dir)
		}
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

	applyMigrations(conn, cfg.Dir(), sqlFiles, cfg, logger)

	logger.Info("clbs-dbtool finished")
}

type sqlFile struct {
	path       string
	hash       string
	apply      bool
	isSnapshot bool
}

// readDir reads the directory recursively and appends all SQL files to the sqlFiles slice
func readDir(files *[]sqlFile, rootDir string, subDir string) error {
	currentDir := path.Join(rootDir, subDir)
	entry, err := os.ReadDir(currentDir)
	if err != nil {
		return err
	}

	allow_snapshot_tag := len(strings.Split(subDir, string(os.PathSeparator))) == 1

	var is_snapshot bool
	var local_files []sqlFile

	for _, e := range entry {
		entryName := e.Name()
		entryPath := path.Join(subDir, entryName)

		// depth first
		if e.IsDir() {
			err := readDir(&local_files, rootDir, entryPath)
			if err != nil {
				return err
			}

			continue
		}

		file_type := getFileType(entryName)

		switch file_type {
		case fileTypeUnknown:
			// if the file has a .sql extension, it's strange a probably a mistake
			if strings.HasSuffix(entryName, ".sql") {
				return fmt.Errorf("the file name '%s' which has .sql extension contains invalid characters", entryName)
			}
			// Non .sql files are just skipped
			continue

		case fileTypeSnapshot:
			if !allow_snapshot_tag {
				return fmt.Errorf(".snapshot file can only be placed in the first level directory, invalid location: %s", entryPath)
			}
			is_snapshot = true
			continue

		case fileTypeSql:
		}

		fileHash, err := getFileHash(path.Join(rootDir, entryPath))
		if err != nil {
			return err
		}

		local_files = append(local_files, sqlFile{path: entryPath, hash: fileHash,
			apply: false,
		})
	}

	if is_snapshot {
		for idx := range local_files {
			local_files[idx].isSnapshot = true
		}
	}

	*files = append(*files, local_files...)

	return nil
}

// isValidFileName checks if the file name is valid
func getFileType(name string) fileType {
	if reFilename.MatchString(name) {
		return fileTypeSql
	}
	if name == ".snapshot" {
		return fileTypeSnapshot
	}
	return fileTypeUnknown
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
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS public.clbs_dbtool_migrations (
			id BIGSERIAL PRIMARY KEY,
			app_id VARCHAR(64) NOT NULL,
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

func isInitialState(conn pgx.Conn, cfg *config.Config) (bool, error) {
	query := `SELECT 1 FROM public.clbs_dbtool_migrations WHERE app_id = $1`
	err := conn.QueryRow(context.Background(), query, cfg.AppId()).Scan(nil)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	return false, err
}

func prepareListOfMigrations(conn pgx.Conn, files []sqlFile, cfg *config.Config) error {
	type migration struct {
		filePath string
		fileHash string
	}

	//goland:noinspection SqlResolve
	selectMigrationsSQL := `SELECT file_path, file_hash FROM public.clbs_dbtool_migrations WHERE app_id = $1 ORDER BY id ASC`

	rows, err := conn.Query(context.Background(), selectMigrationsSQL, cfg.AppId())
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

func applyMigrations(conn *pgx.Conn, rootDir string, files []sqlFile, cfg *config.Config, logger *zap.Logger) {
	//goland:noinspection SqlResolve
	insertExecutedMigrationSQL := `INSERT INTO public.clbs_dbtool_migrations (file_path, file_hash, app_id, clbs_dbtool_version) VALUES ($1, $2, $3, $4)`

	for _, f := range files {
		if !f.apply {
			continue
		}

		logger.Info("Running migration...", zap.String("file", f.path))

		fd, err := os.Open(path.Join(rootDir, f.path))
		if err != nil {
			logger.Fatal("Could not open migration file", zap.Error(err))
		}

		sql, err := readText(fd)
		if err != nil {
			logger.Fatal("Could not read text from migration file", zap.Error(err))
		}

		_, err = conn.Exec(context.Background(), sql)
		if err != nil {
			logger.Fatal("Error while executing migration", zap.Error(err))
		}

		_, err = conn.Exec(context.Background(), insertExecutedMigrationSQL, f.path, f.hash, cfg.AppId(), cfg.Version())
		if err != nil {
			logger.Fatal("Error while updating dbtool migrations table, this may lead to inconsistent database state", zap.Error(err))
		}
	}
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

func prepareFiles(sqlFiles []sqlFile) {
	cache := make(map[string][]string)

	var splitCached = func(s string) []string {
		if t, ok := cache[s]; ok {
			return t
		} else {
			t := strings.Split(s, string(os.PathSeparator))
			cache[s] = t
			return t
		}
	}

	slices.SortFunc(sqlFiles, func(a, b sqlFile) int {
		si := splitCached(a.path)
		sj := splitCached(b.path)
		li := len(si)
		lj := len(sj)

		minLen := min(li-1, lj-1)
		if minLen > 0 {
			for k := range li {
				c := strings.Compare(si[k], sj[k])
				if c != 0 {
					return c
				}
			}
		}

		if li == lj {
			return strings.Compare(si[li-1], sj[lj-1])
		} else {
			return lj - li
		}
	})
}

func getLastSnapshot(sqlFiles *[]sqlFile) (bool, string) {
	// Find last .snapshot
	lastSnapshotIndex := -1
	lastSnapshotDir := ""
	for idx := len(*sqlFiles) - 1; idx >= 0; idx-- {
		sqlfile := (*sqlFiles)[idx]
		if sqlfile.isSnapshot {
			if lastSnapshotIndex == -1 {
				lastSnapshotDir = strings.SplitN(sqlfile.path, string(os.PathSeparator), 2)[0] + string(os.PathSeparator)
			} else if !strings.HasPrefix(sqlfile.path, lastSnapshotDir) {
				break
			}
			lastSnapshotIndex = idx
		} else if lastSnapshotIndex > -1 {
			break
		}
	}

	// Remove files before the last snapshot
	if lastSnapshotIndex != -1 {
		*sqlFiles = (*sqlFiles)[lastSnapshotIndex:]
		return true, lastSnapshotDir
	}
	return false, ""
}
