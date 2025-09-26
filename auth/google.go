package auth

import (
	"calorie-tracker/models"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
)

var GoogleOauthConfig *oauth2.Config
var JwtSecret []byte

func GoogleLogin(c *gin.Context) {
	state := generateStateOauthCookie(c)
	log.Println("Generated state:", state)
	url := GoogleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent select_account"))
	log.Println("Redirecting to Google:", url)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func GoogleCallback(mongoClient *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("=== CALLBACK HIT ===")
		log.Println("Full URL:", c.Request.URL.String())
		state := c.Query("state")
		log.Println("Query state:", state)
		cookie, err := c.Cookie("oauthstate")
		log.Println("Cookie state:", cookie, "Error:", err)
		if err != nil || state != cookie {
			log.Println("STATE MISMATCH ERROR")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid state parameter"})
			return
		}

		code := c.Query("code")
		log.Println("Code:", code)
		if code == "" {
			log.Println("NO CODE ERROR")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code parameter"})
			return
		}

		token, err := GoogleOauthConfig.Exchange(context.Background(), code)
		if err != nil {
			log.Println("TOKEN EXCHANGE ERROR:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
			return
		}
		log.Println("OAuth token OK, Access Token:", token.AccessToken)

		httpClient := &http.Client{}
		req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
		if err != nil {
			log.Println("USER INFO REQUEST ERROR:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user info request"})
			return
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		resp, err := httpClient.Do(req)
		if err != nil {
			log.Println("USER INFO FETCH ERROR:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
			return
		}
		defer resp.Body.Close()
		log.Println("User info response status:", resp.Status)

		userInfo, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println("READ USER INFO ERROR:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read user info"})
			return
		}
		log.Println("Raw user info response:", string(userInfo))

		var googleUser struct {
			Sub           string `json:"sub" bson:"sub"`
			ID            string `json:"id" bson:"id"`
			Email         string `json:"email" bson:"email"`
			Name          string `json:"name" bson:"name"`
			VerifiedEmail bool   `json:"verified_email" bson:"verified_email"`
			GivenName     string `json:"given_name" bson:"given_name"`
		}
		if err := json.Unmarshal(userInfo, &googleUser); err != nil {
			log.Println("PARSE USER INFO ERROR:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user info"})
			return
		}
		if googleUser.Sub == "" {
			googleUser.Sub = googleUser.ID
		}
		log.Println("Parsed user: sub =", googleUser.Sub, "email =", googleUser.Email, "name =", googleUser.Name)

		if googleUser.Sub == "" || googleUser.Email == "" {
			log.Println("INVALID USER INFO: sub =", googleUser.Sub, "email =", googleUser.Email)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user info: missing ID or email"})
			return
		}

		collection := mongoClient.Database("sso").Collection("users")
		var user models.User
		err = collection.FindOne(context.Background(), bson.M{"email": googleUser.Email}).Decode(&user)
		if err != nil {
			log.Println("Creating new user for email:", googleUser.Email)
			user = models.User{
				GoogleID: googleUser.Sub,
				Email:    googleUser.Email,
				Name:     googleUser.Name,
			}
			if user.Name == "" {
				user.Name = googleUser.GivenName
			}
			result, err := collection.InsertOne(context.Background(), user)
			if err != nil {
				log.Println("INSERT USER ERROR:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save user"})
				return
			}
			user.ID = result.InsertedID.(primitive.ObjectID)
			log.Println("Created user ID:", user.ID.Hex())
		} else if user.GoogleID == "" {
			log.Println("EMAIL CONFLICT:", googleUser.Email)
			c.JSON(http.StatusConflict, gin.H{"error": "Email registered with password. Use email login."})
			return
		}

		tokenString, err := GenerateJWT(user.ID.Hex())
		if err != nil {
			log.Println("JWT GENERATION ERROR:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}
		log.Println("JWT generated:", tokenString[:20]+"...")

		session := models.Session{
			UserID:    user.ID,
			Token:     tokenString,
			ExpiresAt: time.Now().Add(time.Hour * 24).Unix(),
		}
		_, err = mongoClient.Database("sso").Collection("sessions").InsertOne(context.Background(), session)
		if err != nil {
			log.Println("SESSION INSERT ERROR:", err)
		}

		redirectURL := "https://calorie-tracker-frontend-ebon.vercel.app/?token=" + tokenString
		log.Println("REDIRECTING TO:", redirectURL)
		c.Redirect(http.StatusFound, redirectURL)
		log.Println("=== CALLBACK END ===")
	}
}

func generateStateOauthCookie(c *gin.Context) string {
	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)
	domain := "localhost"
	if strings.Contains(c.Request.Host, "onrender.com") {
		domain = "calorie-tracker-backend-6nfn.onrender.com"
	}
	c.SetCookie("oauthstate", state, 7200, "/", domain, false, false)
	log.Println("Set oauthstate cookie:", state, "Domain:", domain)
	return state
}