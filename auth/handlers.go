package auth

import (
	"calorie-tracker/models"
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

func Register(client *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var user models.User
		if err := c.BindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		collection := client.Database("sso").Collection("users")
		var existingUser models.User
		err := collection.FindOne(context.Background(), bson.M{"email": user.Email}).Decode(&existingUser)
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		user.Password = string(hashedPassword)

		result, err := collection.InsertOne(context.Background(), user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register"})
			return
		}

		userID := result.InsertedID.(primitive.ObjectID).Hex()
		token, err := GenerateJWT(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		session := models.Session{
			UserID:    result.InsertedID.(primitive.ObjectID),
			Token:     token,
			ExpiresAt: time.Now().Add(time.Hour * 24).Unix(),
		}
		client.Database("sso").Collection("sessions").InsertOne(context.Background(), session)

		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}

func Login(client *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var creds struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.BindJSON(&creds); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		collection := client.Database("sso").Collection("users")
		var user models.User
		err := collection.FindOne(context.Background(), bson.M{"email": creds.Email}).Decode(&user)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		if user.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Use Google login for this account"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(creds.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		token, err := GenerateJWT(user.ID.Hex())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		session := models.Session{
			UserID:    user.ID,
			Token:     token,
			ExpiresAt: time.Now().Add(time.Hour * 24).Unix(),
		}
		client.Database("sso").Collection("sessions").InsertOne(context.Background(), session)

		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}
