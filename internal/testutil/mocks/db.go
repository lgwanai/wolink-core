package mocks

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// NewMockDB creates a GORM DB instance backed by sqlmock for testing.
// The mock expects all queries to be explicitly defined in tests.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    db, mock, err := mocks.NewMockDB(t)
//	    // Define expectations on mock
//	    mock.ExpectQuery("SELECT").WillReturnRows(...)
//	    // Use db as *gorm.DB
//	}
func NewMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, error) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		return nil, nil, err
	}

	// Register cleanup
	t.Cleanup(func() {
		sqlDB.Close()
	})

	// Use MySQL dialector for compatibility with sqlmock
	dialector := mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	})

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, nil, err
	}

	return db, mock, nil
}
