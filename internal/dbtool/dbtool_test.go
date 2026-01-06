package dbtool

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrder(t *testing.T) {
	var sqlFiles []sqlFile
	err := readDir(&sqlFiles, filepath.Join("..", "..", "testing", "samples", "test-dir"), "")
	assert.NoError(t, err)

	prepareFiles(sqlFiles)

	ref := []string{
		"subdir/0000001-init.sql",
		"subdir/file.sql",
		"subdir/file2.sql",
		"subdir/franta.sql",
		"subdir2/eagle_has_landed.sql",
		"subdir2/raymond-reddington.sql",
		"subdir3/subsubdir/alice.sql",
		"subdir3/subsubdir/tobuscus/0-go-go-go.sql",
		"subdir3/subsubdir/tobuscus/00003-upgrade.sql",
		"subdir3/subsubdir/tobuscus/hive.sql",
		"subdir3/subsubdir/umbrelacorp.sql",
		"subdir4/init1.sql",
		"subdir4/init2.sql",
		"subdir5/init1.sql",
		"subdir5/init2.sql",
		"subdir6/justanother.sql",
	}

	assert.Equal(t, len(ref), len(sqlFiles))
	for i, f := range sqlFiles {
		assert.Equal(t, ref[i], f.path)
	}
}

func TestSnapshots(t *testing.T) {
	var sqlFiles []sqlFile
	err := readDir(&sqlFiles, filepath.Join("..", "..", "testing", "samples", "test-dir"), "")
	assert.NoError(t, err)

	prepareFiles(sqlFiles)
	getLastSnapshot(&sqlFiles)

	ref := []string{
		"subdir5/init1.sql",
		"subdir5/init2.sql",
		"subdir6/justanother.sql",
	}

	assert.Equal(t, len(ref), len(sqlFiles))
	for i, f := range sqlFiles {
		assert.Equal(t, ref[i], f.path)
	}
}

func TestGetFileType(t *testing.T) {
	t.Run("Valid SQL filenames", func(t *testing.T) {
		validNames := []string{
			"migration.sql",
			"001-init.sql",
			"0000001-create-table.sql",
			"my_migration.sql",
			"0-start.sql",
			"a.sql",
			"test-123_abc.sql",
		}
		for _, name := range validNames {
			result := getFileType(name)
			assert.Equal(t, fileTypeSql, result, "Expected %s to be valid SQL file", name)
		}
	})

	t.Run("Invalid SQL filenames", func(t *testing.T) {
		invalidNames := []string{
			"Migration.sql",      // uppercase
			"001 init.sql",       // space
			"test@migration.sql", // special char
			".sql",               // starts with dot
			"test.SQL",           // uppercase extension
			"test$.sql",          // special char
		}
		for _, name := range invalidNames {
			result := getFileType(name)
			assert.Equal(t, fileTypeUnknown, result, "Expected %s to be invalid", name)
		}
	})

	t.Run("Snapshot file", func(t *testing.T) {
		result := getFileType(".snapshot")
		assert.Equal(t, fileTypeSnapshot, result)
	})

	t.Run("Non-SQL files", func(t *testing.T) {
		nonSqlFiles := []string{
			"readme.txt",
			"migration.json",
			"test.md",
			"data.csv",
		}
		for _, name := range nonSqlFiles {
			result := getFileType(name)
			assert.Equal(t, fileTypeUnknown, result, "Expected %s to be unknown type", name)
		}
	})
}

func TestGetFileHash(t *testing.T) {
	t.Run("Hash of existing file", func(t *testing.T) {
		testFile := filepath.Join("..", "..", "testing", "samples", "valid", "001_valid.sql")
		hash, err := getFileHash(testFile)
		assert.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.Len(t, hash, 64) // SHA256 produces 64 hex characters
	})

	t.Run("Hash is consistent", func(t *testing.T) {
		testFile := filepath.Join("..", "..", "testing", "samples", "valid", "001_valid.sql")
		hash1, err1 := getFileHash(testFile)
		hash2, err2 := getFileHash(testFile)
		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.Equal(t, hash1, hash2)
	})

	t.Run("Non-existent file returns error", func(t *testing.T) {
		_, err := getFileHash("/non/existent/file.sql")
		assert.Error(t, err)
	})
}

func TestReadText(t *testing.T) {
	t.Run("Read plain text", func(t *testing.T) {
		content := "SELECT * FROM users;"
		reader := strings.NewReader(content)
		result, err := readText(reader)
		assert.NoError(t, err)
		assert.Equal(t, content, result)
	})

	t.Run("Read text with UTF-8 BOM", func(t *testing.T) {
		// UTF-8 BOM is EF BB BF
		contentWithBOM := "\xEF\xBB\xBFSELECT * FROM users;"
		expectedContent := "SELECT * FROM users;"
		reader := strings.NewReader(contentWithBOM)
		result, err := readText(reader)
		assert.NoError(t, err)
		assert.Equal(t, expectedContent, result)
	})

	t.Run("Read multiline text", func(t *testing.T) {
		content := "CREATE TABLE users (\n  id SERIAL PRIMARY KEY,\n  name VARCHAR(100)\n);"
		reader := strings.NewReader(content)
		result, err := readText(reader)
		assert.NoError(t, err)
		assert.Equal(t, content, result)
	})
}

func TestPrepareFiles(t *testing.T) {
	t.Run("Sort files correctly", func(t *testing.T) {
		files := []sqlFile{
			{path: "z/file.sql"},
			{path: "a/file.sql"},
			{path: "m/file.sql"},
		}
		prepareFiles(files)
		assert.Equal(t, "a/file.sql", files[0].path)
		assert.Equal(t, "m/file.sql", files[1].path)
		assert.Equal(t, "z/file.sql", files[2].path)
	})

	t.Run("Sort nested directories", func(t *testing.T) {
		files := []sqlFile{
			{path: "a/b/c/file.sql"},
			{path: "a/b/file.sql"},
			{path: "a/file.sql"},
		}
		prepareFiles(files)
		// The sorting algorithm puts deeper paths first (directories before parent files)
		assert.Equal(t, "a/b/c/file.sql", files[0].path)
		assert.Equal(t, "a/b/file.sql", files[1].path)
		assert.Equal(t, "a/file.sql", files[2].path)
	})
}

func TestGetLastSnapshot(t *testing.T) {
	t.Run("No snapshots returns false", func(t *testing.T) {
		files := []sqlFile{
			{path: "a/file1.sql", isSnapshot: false},
			{path: "b/file2.sql", isSnapshot: false},
		}
		detected, dir := getLastSnapshot(&files)
		assert.False(t, detected)
		assert.Empty(t, dir)
		assert.Len(t, files, 2)
	})

	t.Run("Single snapshot directory", func(t *testing.T) {
		files := []sqlFile{
			{path: "a/file1.sql", isSnapshot: false},
			{path: "b/file2.sql", isSnapshot: true},
			{path: "b/file3.sql", isSnapshot: true},
			{path: "c/file4.sql", isSnapshot: false},
		}
		detected, dir := getLastSnapshot(&files)
		assert.True(t, detected)
		assert.Equal(t, "b/", dir)
		assert.Len(t, files, 3) // b/file2.sql, b/file3.sql, c/file4.sql
	})
}
