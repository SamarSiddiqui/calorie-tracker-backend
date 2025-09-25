package main

import (
	"calorie-tracker/auth"
	"calorie-tracker/db"
	"calorie-tracker/services"
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	jwtSecret := os.Getenv("JWT_SECRET")
	mongoURI := os.Getenv("MONGODB_URI")
	callbackURL := os.Getenv("CALLBACK_URL")

	if googleClientID == "" || googleClientSecret == "" || jwtSecret == "" || mongoURI == "" {
		log.Fatal("Missing environment variables")
	}
	log.Println("Environment variables loaded:", googleClientID, mongoURI, "Callback URL:", callbackURL)

	auth.JwtSecret = []byte(jwtSecret)

	auth.GoogleOauthConfig = &oauth2.Config{
		ClientID:     googleClientID,
		ClientSecret: googleClientSecret,
		RedirectURL:  callbackURL + "/auth/google/callback",
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
		AllowOrigins:     []string{"http://localhost:5174", "https://your-app.vercel.app"}, // Add production URL
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.Static("/static", "./frontend/dist")
	r.POST("/register", auth.Register(client))
	r.POST("/login", auth.Login(client))
	r.GET("/auth/google/login", auth.GoogleLogin)
	r.GET("/auth/google/callback", auth.GoogleCallback(client))
	r.POST("/calories/add", auth.AuthMiddleware(), services.AddCalorie(client))
	r.GET("/calories/view", auth.AuthMiddleware(), services.ViewCalories(client))
	r.DELETE("/calories/delete/:id", auth.AuthMiddleware(), services.DeleteCalorie(client))

	// SPA routing fallback
	r.NoRoute(func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})

	log.Println("Starting server on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
