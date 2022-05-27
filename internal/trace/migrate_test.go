package trace

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrate(t *testing.T) {
	db := emptyDB()
	defer db.Close()

	err := Migrate(db)
	require.NoError(t, err)
}
