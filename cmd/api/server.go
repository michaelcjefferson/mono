package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Create and serve an httpserver based on app parameters. Before starting the server, run a goroutine that listens for Interrupt and Terminate signals, and attempts to shut down the server and any background goroutines gracefully.
func (app *application) serve() error {
	tlsConfig := &tls.Config{
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", app.config.Server.Port),
		Handler: app.routes(),
		// Create a logger to be used as standard error, so that error logs coming from http.server, which is configured to print to stderror rather than stdout, will be caught and sent to the custom jsonlog logger instead.
		ErrorLog:     log.New(app.logger, "", 0),
		TLSConfig:    tlsConfig,
		ReadTimeout:  app.config.Server.ReadTimeout,
		IdleTimeout:  app.config.Server.IdleTimeout,
		WriteTimeout: app.config.Server.WriteTimeout,
	}

	shutdownError := make(chan error)

	// Create a goroutine that listens for program-ending signals in the background, and instead of just causing the server to stop without completing requests, writes etc., allows the server to gracefully shut down.
	go func() {
		// Set up a channel that receives an os.Signal, with a buffer of 1 - the buffer of 1 means that s := <-quit will wait (buffer) until the quit channel has received a value to send to s.
		quit := make(chan os.Signal, 1)

		// Listen for SIGINT (ctrl+c) and SIGTERM signals, and on either event, send the signal to the quit channel, triggering the buffer s below.
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// This will block until a signal is received
		s := <-quit

		app.logger.Info("shutting down server", map[string]any{
			"signal": s.String(),
		})

		close(app.isShuttingDown)

		// Create a context with a 5 second timeout. This is due to an open bug here: https://github.com/golang/go/issues/33191, which causes problems above 5 seconds - otherwise, something like 20 seconds would be more appropriate.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// If Shutdown() returns an error, send it to shutdownError.
		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownError <- err
		}

		app.logger.Info("completing background tasks", map[string]any{
			"addr": srv.Addr,
		})

		// Wait for all goroutines in WaitGroup to complete before sending nil to shutdownError
		app.wg.Wait()
		shutdownError <- err
	}()

	app.initiateTokenDeletionCycle()

	app.logger.Info("server started", map[string]any{
		"addr": srv.Addr,
		"env":  app.config.Project.Env,
	})

	// Shutdown causes an ErrServerClosed to be thrown, which is the desired outcome - only return an error that is not of this type.
	var err error
	if app.config.Server.TLS.HTTPSOn {
		err = srv.ListenAndServeTLS(app.config.Server.TLS.CertPath, app.config.Server.TLS.KeyPath)
	} else {
		err = srv.ListenAndServe()
	}
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Successful Shutdown returns nil, so if shutdownError is not nil, return the error that occurred.
	err = <-shutdownError
	if err != nil {
		return err
	}

	// Otherwise, Shutdown completed successfully - log that fact, and return nil.
	app.logger.Info("stopped server", map[string]any{
		"addr": srv.Addr,
	})

	return nil
}
