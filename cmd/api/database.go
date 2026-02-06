package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"placeholder_project_tag/internal/config"
	"placeholder_project_tag/pkg/apperrors"
	dbbackup "placeholder_project_tag/pkg/db-backup"

	"github.com/labstack/echo/v4"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed app/migrations/*.sql
var appDBMigrations embed.FS

//go:embed monitor/migrations/*.sql
var monitorDBMigrations embed.FS

// SQLite config for reads and writes (avoid SQLITE BUSY error): https://kerkour.com/sqlite-for-servers
// Create string of connection params to prevent "SQLITE_BUSY" errors - to be further improved based on the above article
var dbParams string = "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_parseTime=on&_foreign_keys=on"

func openAppDB(cfg *config.Config) (*sql.DB, error) {
	// Either connect to or create (if it doesn't exist) the database at the provided path
	db, err := sql.Open("sqlite3", cfg.DB.AppDBPath+dbParams)
	if err != nil {
		return nil, err
	}

	// Create context with 5 second deadline so that we can ping the db and finish establishing a db connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	runMigrations(db, "app")

	return db, nil
}

func openMonitorDB(cfg *config.Config) (*sql.DB, error) {
	// Either connect to or create (if it doesn't exist) the database at the provided path
	log.Printf("db connection string: %s", cfg.DB.MonitorDBPath+dbParams)
	db, err := sql.Open("sqlite3", cfg.DB.MonitorDBPath+dbParams)
	if err != nil {
		log.Printf("returning with error from sql.Open monitor db: %v", err)
		return nil, err
	}

	// Create context with 5 second deadline so that we can ping the db and finish establishing a db connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		log.Printf("returning with error from PingContext (monitor db): %v", err)
		return nil, err
	}

	runMigrations(db, "monitor")

	return db, nil
}

// Apply migrations from embedded folder
func runMigrations(db *sql.DB, dbType string) error {
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return err
	}

	var source source.Driver

	if dbType == "app" {
		source, err = iofs.New(appDBMigrations, "app/migrations")
	} else if dbType == "monitor" {
		source, err = iofs.New(monitorDBMigrations, "monitor/migrations")
	} else {
		return errors.New("provided db doesn't exist")
	}
	if err != nil {
		log.Printf("returning from run migrations (monitor) with error: %v", err)
		return err
	}

	m, err := migrate.NewWithInstance(
		"iofs",
		source,
		"sqlite3",
		driver,
	)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

// TODO: add backup of monitor.db
// Create copy of app.db in db/backups on startup and once every 6 hours
func (app *application) initiateDBBackupCycle() {
	app.background(func() {
		dst, err := dbbackup.RunBackup(app.config.DB.AppDBPath)
		if err != nil {
			app.logger.Error(err, map[string]any{
				"action": "backup db on startup",
			})
		} else {
			app.logger.Info("db backed up", map[string]any{
				"backup_path": dst,
			})
		}

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				dst, err := dbbackup.RunBackup(app.config.DB.AppDBPath)
				if err != nil {
					app.logger.Error(err, map[string]any{
						"action": "backup db in background process",
					})
				} else {
					app.logger.Info("db backed up", map[string]any{
						"backup_path": dst,
					})
				}
			case <-app.isShuttingDown:
				app.logger.Info("db backup cycle ending - shut down signal received", nil)
				return
			}
		}
	})
}

func (app *application) backupDBHandler(c echo.Context) error {
	u := app.contextGetUser(c)

	app.logger.Info("triggered database backup", map[string]any{
		"user_id": u.ID,
	})

	dst, err := dbbackup.RunBackup(app.config.DB.AppDBPath)
	if err != nil {
		app.logger.Error(err, map[string]any{
			"action": "backup db from user request",
		})
		return app.errorAPIResponse(c, err, apperrors.ErrCodeInternalServer, map[string]any{"message": "failed to backup database"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": fmt.Sprintf("database backed up successfully - %s", dst),
	})
}
