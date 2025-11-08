package dbtool

import (
	"path/filepath"
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
