package migrations

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCommandClassificationMigrationPreservesExistingAuditRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, db.Exec("CREATE TABLE audit_logs (id INTEGER PRIMARY KEY, command TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO audit_logs (id,command) VALUES (1, 'uname -a')").Error)
	require.NoError(t, migration202610030001().Migrate(db))
	require.NoError(t, migration202610030001().Migrate(db))
	var row struct{ Command, CommandType, Classification string }
	require.NoError(t, db.Table("audit_logs").First(&row).Error)
	require.Equal(t, "uname -a", row.Command)
	require.Empty(t, row.CommandType)
	require.NoError(t, db.Exec("UPDATE audit_logs SET command_type = 'SAFE_READ', classification = '{}' WHERE id = 1").Error)
	require.NoError(t, db.Table("audit_logs").First(&row).Error)
	require.Equal(t, "SAFE_READ", row.CommandType)
}
