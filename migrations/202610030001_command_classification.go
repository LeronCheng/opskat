package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func migration202610030001() *gormigrate.Migration {
	return &gormigrate.Migration{ID: "202610030001", Migrate: func(tx *gorm.DB) error {
		for _, column := range []struct{ name, definition string }{
			{"command_type", "VARCHAR(40) DEFAULT ''"},
			{"classification", "TEXT DEFAULT ''"},
		} {
			if !tx.Migrator().HasColumn("audit_logs", column.name) {
				if err := tx.Exec("ALTER TABLE audit_logs ADD COLUMN " + column.name + " " + column.definition).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}}
}
