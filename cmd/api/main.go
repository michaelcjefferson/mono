package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"placeholder_project_tag/internal/config"
	"placeholder_project_tag/internal/data"
	"placeholder_project_tag/pkg/logging"

	"golang.org/x/oauth2"

	// Allows struct tags that read .env vars into struct fields, with features such as default values and required env vars

	// Provide mem/cpu leak analysis at /debug/pprof

	"net/http"
	_ "net/http/pprof"

	// go-sqlite3 isn't being used directly in this file - it is instead registered with database/sql. So, alias go-sqlite3 import to _ to prevent the Go compiler from complaining that it isn't being used
	"github.com/caarlos0/env/v10"
	_ "github.com/mattn/go-sqlite3"
)

type application struct {
	config     *config.Config
	googleAuth *oauth2.Config
	// Allows processes, eg. token deletion cycle, to respond to this channel closing (and eg. perform tidy up operations)
	isShuttingDown chan struct{}
	logger         *logging.Logger
	models         data.Models
	wg             sync.WaitGroup
}

func main() {
	cfg := &config.Config{}

	if err := env.Parse(cfg); err != nil {
		panic(fmt.Sprintf("couldn't load config from env vars: %s", err))
	}

	cfg.Print()

	if err := cfg.Validate(); err != nil {
		panic(fmt.Sprintf("config loaded from env vars failed validation: %s", err))
	}

	app := &application{
		config:         cfg,
		isShuttingDown: make(chan struct{}),
	}

	if cfg.IsDevelopment() || cfg.IsStaging() {
		// Expose pprof endpoints on port 6060 as a separate service
		go func() {
			http.ListenAndServe("localhost:6060", nil)
		}()
	}

	if err := app.createFileDirs(); err != nil {
		log.Fatalf("couldn't set up app data directories: %v", err)
	}

	fmt.Println("attempting to set up monitoring db")

	monitorDB, err := openMonitorDB(cfg)
	if monitorDB == nil {
		if err != nil {
			log.Printf("error returned from openMonitorDB: %v\n", err)
		}
		dirs, err := os.ReadDir(".")
		if err != nil {
			log.Printf("error reading dirs: %v", err)
		}
		for _, dir := range dirs {
			log.Printf("found: %v\nis dir: %v", dir.Name(), dir.IsDir())
		}
		panic(fmt.Sprintf("couldn't set up monitor db\ndb path: %s", cfg.DB.MonitorDBPath))
	}
	if err != nil {
		log.Printf("error setting up monitor database: %v\n", err)
		time.Sleep(20 * time.Second)
	}
	defer monitorDB.Close()

	fmt.Println("attempting to set up app db")

	appDB, err := openAppDB(cfg)
	if appDB == nil {
		if err != nil {
			log.Printf("error returned from openAppDB: %v\n", err)
		}
		dirs, err := os.ReadDir(".")
		if err != nil {
			log.Printf("error reading dirs: %v", err)
		}
		for _, dir := range dirs {
			log.Printf("found: %v", dir.Name())
		}
		panic(fmt.Sprintf("couldn't set up app db\ndb path: %s", cfg.DB.AppDBPath))
	}
	if err != nil {
		log.Printf("error setting up app database: %v\n", err)
		time.Sleep(20 * time.Second)
	}
	defer appDB.Close()

	models := data.NewModels(appDB, monitorDB)
	app.models = models

	if app.config.Logging.LogToDB {
		app.logger = logging.New(
			os.Stdout,
			logging.WithDatabase(&app.models.Logs),
			logging.WithService("api"),
			logging.WithMinLevel(logging.LevelInfo))
	} else {
		app.logger = logging.New(
			os.Stdout,
			logging.WithService("api"),
			logging.WithMinLevel(logging.LevelInfo),
		)
	}

	app.logger.Info("database connection pool established", nil)

	if app.config.DB.BackUpEnabled {
		// Initiated here rather than in app.serve() to run backup before any other actions
		app.initiateDBBackupCycle()
	}

	// app.googleAuth = &oauth2.Config{
	// 	ClientID:     app.config.Google.GoogleClientID,
	// 	ClientSecret: app.config.Google.GoogleClientSecret,
	// 	RedirectURL:  app.config.Google.GoogleRedirectURL,
	// 	Scopes:       []string{"email", "profile"},
	// 	Endpoint:     google.Endpoint,
	// }

	err = app.serve()
	if err != nil {
		app.logger.Fatal(err, nil)
	}
}
