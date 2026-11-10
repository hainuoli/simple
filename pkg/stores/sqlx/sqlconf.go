// Package sqlx is a thin, reflection-free layer over database/sql.
//
// It provides connection configuration, typed row scanning, and transaction
// helpers. It intentionally does not build SQL, map struct fields at runtime,
// or hide database/sql behavior.
package sqlx

import "time"

// SqlConf defines how a database connection pool is created.
//
// DriverName and DataSource are passed to sql.Open unchanged. Pool is optional:
// a nil Pool keeps the database/sql defaults.
type SqlConf struct {
	DriverName string
	DataSource string
	Pool       *PoolConf
}

// PoolConf defines database/sql pool settings.
//
// Zero values keep the standard library semantics:
//   - MaxOpenConns == 0 means no limit.
//   - MaxIdleConns == 0 means no idle connections are retained.
//   - ConnMaxLifetime == 0 means connections are not expired by age.
//   - ConnMaxIdleTime == 0 means idle connections are not expired by time.
type PoolConf struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}
