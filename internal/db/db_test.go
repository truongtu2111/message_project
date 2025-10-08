package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew_InvalidDatabaseURL(t *testing.T) {
	// Test with invalid database URL
	db, err := New("invalid-url")

	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "failed to ping database")
}

func TestNew_ValidDatabaseURL(t *testing.T) {
	// Skip this test if no test database is available
	t.Skip("Requires actual database connection for integration testing")

	// This would be used in integration tests with a real database
	// db, err := New("postgres://user:password@localhost/testdb?sslmode=disable")
	// assert.NoError(t, err)
	// assert.NotNil(t, db)
	// defer db.Close()
}

func TestDB_Close(t *testing.T) {
	// Skip this test if no test database is available
	t.Skip("Requires actual database connection for integration testing")

	// This would be used in integration tests with a real database
	// db, err := New("postgres://user:password@localhost/testdb?sslmode=disable")
	// require.NoError(t, err)
	//
	// err = db.Close()
	// assert.NoError(t, err)
}
