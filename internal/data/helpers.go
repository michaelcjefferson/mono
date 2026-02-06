package data

import (
	"log"
	"reflect"
	"strconv"
	"strings"

	"github.com/mattn/go-sqlite3"
)

// OTHER HELPERS
// Tries to read and convert an any value (eg. the ones in log.Details) to an int value - returns an int (or 0 on failure) and a bool (ok)
func ToInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int8, int16, int32, int64:
		return int(reflect.ValueOf(v).Int()), true
	case uint, uint8, uint16, uint32, uint64:
		return int(reflect.ValueOf(v).Uint()), true
	case float32, float64:
		return int(reflect.ValueOf(v).Float()), true
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	}
	return 0, false
}

// Creates a string of comma separated ? placeholders based on the int provided - good for dynamically adding placeholders to a query based on the number of parameters
func Placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// Utility function for the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ProcessSQLError(err error, msg string) error {
	log.Printf("%s: %s\n\n", msg, err)
	if sqliteErr, ok := err.(sqlite3.Error); ok {
		if sqliteErr.Code == sqlite3.ErrConstraint && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return ErrDuplicateEntry
		} else {
			return sqliteErr
		}
	} else {
		return err
	}
}
