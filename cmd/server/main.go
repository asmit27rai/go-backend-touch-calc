package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"math/rand"
	"time"

	"github.com/c4gt/tornado-nginx-go-backend/internal/config"
	"github.com/c4gt/tornado-nginx-go-backend/internal/handlers"
	"github.com/c4gt/tornado-nginx-go-backend/pkg/middleware"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())
	
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Load configuration
	cfg := config.Load()

	// Debug: Print configuration
	log.Printf("Storage backend: %s", cfg.StorageBackend)
	log.Printf("MongoDB URI: %s", cfg.MongoURI)
	log.Printf("MySQL DSN: %s", cfg.MySQLDSN)

	// Initialize Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()

	// Apply middleware
	router.Use(middleware.CORS())
	router.Use(middleware.Logger())
	router.Use(middleware.Recovery())

	// Initialize handlers
	handler := handlers.NewHandler(cfg)

	// Setup routes
	setupRoutes(router, handler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Printf("Storage backend: %s", cfg.StorageBackend)
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func setupRoutes(router *gin.Engine, handler *handlers.Handler) {
	// Static files with proper paths
	router.Static("/static", "./web/static")
	router.StaticFS("/js", http.Dir("./web/static/js"))
	router.StaticFS("/css", http.Dir("./web/static/css"))
	router.StaticFS("/images", http.Dir("./web/static/images"))

	// Load HTML templates with error handling
	templatePattern := "web/templates/*"
	files, err := filepath.Glob(templatePattern)
	if err != nil || len(files) == 0 {
		log.Printf("WARNING: No template files found matching pattern %s", templatePattern)
		log.Printf("Creating fallback template to prevent panic...")
		// Create a simple fallback template to prevent panic
		fallbackTemplate := template.Must(template.New("fallback").Parse(`
<!DOCTYPE html><html><head><title>TouchCalc Backend</title></head>
<body><h1>TouchCalc Backend is Running</h1><p>Template system is loading...</p>
<a href="/health">Check Health</a></body></html>`))
		router.SetHTMLTemplate(fallbackTemplate)
	} else {
		log.Printf("Loading %d template files from %s", len(files), templatePattern)
		for _, file := range files {
			log.Printf(" - %s", file)
		}
		router.LoadHTMLGlob(templatePattern)
	}

	// Health check endpoint (define this early)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":           "healthy",
			"service":          "tornado-nginx-go-backend",
			"storage":          handler.Config.StorageBackend,
			"templates_loaded": len(files),
		})
	})

	// API routes
	api := router.Group("/")
	{
		// Home route - matches Flask behavior exactly
		api.GET("/", func(c *gin.Context) {
			user := getCurrentUser(c)
			if user == "" {
				c.Redirect(http.StatusFound, "/login")
			} else {
				c.Redirect(http.StatusFound, "/save")
			}
		})

		// Authentication routes
		api.POST("/iauth", handler.Auth.HandleAuth)
		api.GET("/login", handler.Auth.HandleLoginGet)
		api.POST("/login", handler.Auth.HandleLogin)
		api.GET("/register", handler.Auth.HandleRegisterGet)
		api.POST("/register", handler.Auth.HandleRegister)
		api.GET("/logout", handler.Auth.HandleLogout)
		api.POST("/logout", handler.Auth.HandleLogout)
		api.GET("/pwreset", handler.Auth.HandlePasswordResetGet)
		api.POST("/pwreset", handler.Auth.HandlePasswordResetPost)
		api.GET("/lostpw", handler.Auth.HandleLostPassword)
		api.POST("/lostpw", handler.Auth.HandleLostPassword)

		// NEW FLASK-COMPATIBLE ROUTES
		api.GET("/save", handler.WebApp.HandleSave)
		api.POST("/save", handler.WebApp.HandleSave)
		api.POST("/usersheet", handler.WebApp.HandleUserSheet)
		api.GET("/import", handler.WebApp.HandleImportGet)
		api.POST("/import", handler.WebApp.HandleImportPost)
		api.POST("/downloadfile", handler.WebApp.HandleDownloadFile)
		api.GET("/htmltopdf", handler.WebApp.HandleHTMLToPDFGet)
		api.POST("/htmltopdf", handler.WebApp.HandleHTMLToPDFPost)

		// Existing web app routes
		api.POST("/iwebapp", handler.WebApp.HandleWebApp)

		// Email routes
		api.POST("/irunasemailer", handler.Email.HandleRunAsEmail)

		// Browser/app routes (existing)
		api.GET("/browser", handler.App.HandleLanding)
		api.GET("/browser/:param1/:paramCode/:param2", handler.App.HandleAmazonWebApp)
		api.GET("/browser/:param1/dropbox", handler.Dropbox.HandleDropboxGet)
		api.POST("/browser/:param1/dropbox", handler.Dropbox.HandleDropboxPost)
		api.GET("/browser/static/*filepath", handler.App.HandleGoogleVerification)
	}
}

// Helper function to get current user from cookie
func getCurrentUser(c *gin.Context) string {
	userCookie, err := c.Cookie("user")
	if err != nil {
		return ""
	}
	// Handle both JSON format and plain text format
	if len(userCookie) > 0 && userCookie[0] == '"' && userCookie[len(userCookie)-1] == '"' {
		// JSON format
		var user string
		err = json.Unmarshal([]byte(userCookie), &user)
		if err != nil {
			return ""
		}
		return user
	}
	// Plain text format
	return userCookie
}
