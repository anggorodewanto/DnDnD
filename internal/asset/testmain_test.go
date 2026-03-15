package asset_test

import (
	"os"
	"testing"

	dbfs "github.com/ab/dndnd/db"
	"github.com/ab/dndnd/internal/testutil"
)

var sharedDB = testutil.NewSharedTestDB(dbfs.Migrations)

func TestMain(m *testing.M) {
	code := m.Run()
	sharedDB.Teardown()
	os.Exit(code)
}
