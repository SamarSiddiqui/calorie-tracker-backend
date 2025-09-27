package main

import (
	"calorie-tracker/auth"
	"calorie-tracker/db"
	"calorie-tracker/services"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables:", err)
	}

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	jwtSecret := os.Getenv("JWT_SECRET")
	mongoURI := os.Getenv("MONGODB_URI")

	if googleClientID == "" || googleClientSecret == "" || jwtSecret == "" || mongoURI == "" {
		log.Fatal("Missing environment variables")
	}

	log.Println("Environment variables loaded:", googleClientID, mongoURI)

	auth.JwtSecret = []byte(jwtSecret)

	redirectURL := "http://localhost:8080/auth/google/callback"
	if os.Getenv("RENDER") == "true" {
		redirectURL = "https://calorie-tracker-backend-6nfn.onrender.com/auth/google/callback"
	}

	auth.GoogleOauthConfig = &oauth2.Config{
		ClientID:     googleClientID,
		ClientSecret: googleClientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
			"openid",
		},
		Endpoint: google.Endpoint,
	}
	log.Println("Google OAuth config initialized:", auth.GoogleOauthConfig.ClientID, "RedirectURL:", auth.GoogleOauthConfig.RedirectURL)

	client, err := db.Connect(mongoURI)
	if err != nil {
		log.Fatal("MongoDB connection failed:", err)
	}
	log.Println("Connected to MongoDB")

	r := gin.Default()
	r.Use(gin.Logger())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5174", "https://calorie-tracker-frontend-ebon.vercel.app"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.POST("/register", auth.Register(client))
	r.POST("/login", auth.Login(client))
	r.GET("/auth/google/login", auth.GoogleLogin)
	r.GET("/auth/google/callback", auth.GoogleCallback(client))
	r.POST("/calories/add", auth.AuthMiddleware(), services.AddCalorie(client))
	r.GET("/calories/view", auth.AuthMiddleware(), services.ViewCalories(client))
	r.DELETE("/calories/delete/:id", auth.AuthMiddleware(), services.DeleteCalorie(client))
	r.PUT("/calories/update/:id", auth.AuthMiddleware(), services.UpdateCalorie(client))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	log.Println("Starting server on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}