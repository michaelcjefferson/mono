package main

import (
	"embed"
	"net/http"
	"placeholder_project_tag/internal/data"

	"golang.org/x/time/rate"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

//go:embed static/css/*
var embeddedCSS embed.FS

//go:embed static/img/*
var embeddedImg embed.FS

//go:embed static/js/*
var embeddedJS embed.FS

func (app *application) routes() http.Handler {
	router := echo.New()

	router.Use(app.recoverPanicMiddleware)
	router.Use(app.requestIDMiddleware())

	csrfMiddleware := middleware.CSRFWithConfig(middleware.CSRFConfig{
		TokenLookup:    "header:X-CSRF-Token",
		CookieName:     "_csrf",
		CookiePath:     "/",
		CookieHTTPOnly: false,
		CookieSecure:   true,
		CookieSameSite: http.SameSiteLaxMode,
		ContextKey:     "csrf",
	})

	// Only use rate limiter if enabled, and use custom values in config
	if app.config.Server.Limiter.Enabled {
		router.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate:  rate.Limit(app.config.Server.Limiter.RPS),
				Burst: app.config.Server.Limiter.Burst,
			},
		)))
	}

	// Ensure static file routes are clean
	router.Pre(middleware.RemoveTrailingSlash())

	// Clean route-file matching by navigating filesystem inside of /static folder - domain.com/css/style.css instead of domain.com/static/css/style.css (css remains in route due to StaticFS("/css/*" below))
	cssFS := echo.MustSubFS(embeddedCSS, "static/css")

	router.StaticFS("/css/*", cssFS)

	imgFS := echo.MustSubFS(embeddedImg, "static/img")
	router.StaticFS("/img/*", imgFS)

	jsFS := echo.MustSubFS(embeddedJS, "static/js")

	router.StaticFS("/js/*", jsFS)

	router.GET("/error", app.errorCheckHandler)
	router.GET("/healthcheck", app.healthcheckHandler)
	// Prevent error when browser attempts to access some common files that don't exist yet. To be populated with files such as security.txt and dns-challenge (for Let's Encrypt)
	router.GET("/.well-known/*", echo.NotFoundHandler)

	// Routes that require authentication - clients accessing these routes will pass through authentication, being set as either the user associated with their cookie or an anonymous user
	// To set middleware on any route branching off of "/", including the "/" route itself, echo requires the route group to be set on "" rather than "/" as it appends a trailing slash.
	web := router.Group("", app.authenticateUser, csrfMiddleware)
	web.GET("/register", app.registerPageHandler)
	web.POST("/register", app.registerUserHandler)

	web.GET("/sign-in", app.signInPageHandler)
	// A route that allows the user to input either their username or their email address
	web.POST("/sign-in", app.signInHandler)

	web.GET("/oauth/google", app.googleLoginRedirectHandler)
	web.GET("/oauth/google/callback", app.googleCallbackHandler)

	// Routes that require the user to be successfully authenticated
	// All routes in this group first pass through authenticateUser, as it is defined on top of the web.Group
	authWeb := web.Group("", app.requireAuthenticatedUser)
	// The user must be authenticated in order to be logged out successfully, and to reach the dashboard
	authWeb.POST("/log-out", app.logoutUserHandler)

	authWeb.GET("/profile/username", app.usernamePageHandler)
	authWeb.POST("/profile/username/update", app.usernameUpdateHandler)

	authWeb.GET("/", app.homepagePageHandler)

	// Routes requiring admin:access permission
	adminWeb := authWeb.Group("/admin", app.requirePermissionCode(data.PermissionAdminAccess))
	adminWeb.GET("", app.adminDashboardHandler)

	// adminWeb.GET("/logs/partial", app.getFilteredLogsHandler)
	adminWeb.GET("/logs/:id", app.getIndividualLogPageHandler)
	adminWeb.GET("/logs", app.getFilteredLogsPageHandler)

	adminWeb.GET("/users", app.getUsersAdminPageHandler)

	// No auth required for this endpoint
	router.POST("/admin/init", app.initialiseAdmin)

	return router
}
