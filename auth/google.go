package auth

import (
	"calorie-tracker/models"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
)

var GoogleOauthConfig *oauth2.Config

func GoogleLogin(c *gin.Context) {
	state := generateStateOauthCookie(c)
	url := GoogleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent select_account"))
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func GoogleCallback(mongoClient *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		state := c.Query("state")
		cookie, err := c.Cookie("oauthstate")
		if err != nil || state != cookie {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid state parameter"})
			return
		}

		code := c.Query("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code parameter"})
			return
		}

		token, err := GoogleOauthConfig.Exchange(context.Background(), code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
			return
		}

		httpClient := &http.Client{}
		req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user info request"})
			return
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		resp, err := httpClient.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
			return
		}
		defer resp.Body.Close()

		userInfo, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read user info"})
			return
		}

		var googleUser struct {
			Sub           string `json:"sub" bson:"sub"`
			ID            string `json:"id" bson:"id"`
			Email         string `json:"email" bson:"email"`
			Name          string `json:"name" bson:"name"`
			VerifiedEmail bool   `json:"verified_email" bson:"verified_email"`
			GivenName     string `json:"given_name" bson:"given_name"`
		}
		if err := json.Unmarshal(userInfo, &googleUser); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse user info"})
			return
		}
		if googleUser.Sub == "" {
			googleUser.Sub = googleUser.ID
		}

		if googleUser.Sub == "" || googleUser.Email == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user info: missing ID or email"})
			return
		}

		collection := mongoClient.Database("sso").Collection("users")
		var user models.User
		err = collection.FindOne(context.Background(), bson.M{"email": googleUser.Email}).Decode(&user)
		if err != nil {
			user = models.User{
				GoogleID: googleUser.Sub,
				Email:    googleUser.Email,
				Name:     googleUser.Name,
			}
			if user.Name == "" {
				user.Name = googleUser.GivenName
			}
			_, err := collection.InsertOne(context.Background(), user)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save user"})
				return
			}
		} else if user.GoogleID == "" {
			c.JSON(http.StatusConflict, gin.H{"error": "Email registered with password. Use email login."})
			return
		}

		tokenString, err := GenerateJWT(user.ID.Hex())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		session := models.Session{
			UserID:    user.ID,
			Token:     tokenString,
			ExpiresAt: time.Now().Add(time.Hour * 24).Unix(),
		}
		_, err = mongoClient.Database("sso").Collection("sessions").InsertOne(context.Background(), session)
		if err != nil {
		}

		redirectURL := "https://calorie-tracker-frontend-ebon.vercel.app/?token=" + tokenString
		c.Redirect(http.StatusFound, redirectURL)
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
	return state
}