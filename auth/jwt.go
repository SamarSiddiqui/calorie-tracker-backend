package auth

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

var JwtSecret []byte

func GenerateJWT(userID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, err := token.SignedString(JwtSecret)
	if err != nil {
		log.Println("JWT SIGNING ERROR:", err)
		return "", err
	}
	return tokenString, nil
}

func ValidateJWT(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Println("INVALID SIGNING METHOD")
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return JwtSecret, nil
	})
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		log.Println("Auth header:", authHeader)
		if authHeader == "" {
			log.Println("MISSING AUTH HEADER")
			c.JSON(401, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			log.Println("INVALID AUTH HEADER FORMAT")
			c.JSON(401, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		token, err := ValidateJWT(parts[1])
		if err != nil {
			log.Println("JWT PARSE ERROR:", err)
			c.JSON(401, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID, ok := claims["user_id"].(string)
			if !ok {
				log.Println("INVALID USERID IN TOKEN")
				c.JSON(401, gin.H{"error": "Invalid user ID in token"})
				c.Abort()
				return
			}
			c.Set("user_id", userID)
			log.Println("JWT validated, userID:", userID)
			c.Next()
		} else {
			log.Println("INVALID TOKEN CLAIMS")
			c.JSON(401, gin.H{"error": "Invalid token claims"})
			c.Abort()
		}
	}
}
