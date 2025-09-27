package services

import (
	"calorie-tracker/models"
	"context"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func AddCalorie(client *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var entry models.CalorieEntry
		if err := c.ShouldBindJSON(&entry); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request payload"})
			return
		}

		userIDStr, exists := c.Get("user_id")
		if !exists {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			return
		}

		userID, err := primitive.ObjectIDFromHex(userIDStr.(string))
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid user ID"})
			return
		}

		entry.UserID = userID
		entry.ID = primitive.NewObjectID()

		collection := client.Database("sso").Collection("calories")
		_, err = collection.InsertOne(context.Background(), entry)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to add calorie entry"})
			return
		}

		c.JSON(200, gin.H{"message": "Calorie entry added"})
	}
}

func ViewCalories(client *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, exists := c.Get("user_id")
		if !exists {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			return
		}

		userID, err := primitive.ObjectIDFromHex(userIDStr.(string))
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid user ID"})
			return
		}

		collection := client.Database("sso").Collection("calories")
		cursor, err := collection.Find(context.Background(), bson.M{"user_id": userID})
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to fetch calories"})
			return
		}
		defer cursor.Close(context.Background())

		var entries []models.CalorieEntry
		if err := cursor.All(context.Background(), &entries); err != nil {
			c.JSON(500, gin.H{"error": "Failed to decode calories"})
			return
		}

		c.JSON(200, entries)
	}
}

func DeleteCalorie(client *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		entryIDStr := c.Param("id")

		entryID, err := primitive.ObjectIDFromHex(entryIDStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid entry ID"})
			return
		}

		userIDStr, exists := c.Get("user_id")
		if !exists {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			return
		}

		userID, err := primitive.ObjectIDFromHex(userIDStr.(string))
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid user ID"})
			return
		}

		collection := client.Database("sso").Collection("calories")
		result, err := collection.DeleteOne(context.Background(), bson.M{
			"_id":     entryID,
			"user_id": userID,
		})
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to delete calorie entry"})
			return
		}

		if result.DeletedCount == 0 {
			c.JSON(404, gin.H{"error": "Entry not found or not owned by user"})
			return
		}

		c.JSON(200, gin.H{"message": "Calorie entry deleted"})
	}
}

func UpdateCalorie(client *mongo.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		entryIDStr := c.Param("id")

		entryID, err := primitive.ObjectIDFromHex(entryIDStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid entry ID"})
			return
		}

		userIDStr, exists := c.Get("user_id")
		if !exists {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			return
		}

		userID, err := primitive.ObjectIDFromHex(userIDStr.(string))
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid user ID"})
			return
		}

		var entry models.CalorieEntry
		if err := c.ShouldBindJSON(&entry); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request payload"})
			return
		}

		collection := client.Database("sso").Collection("calories")
		result, err := collection.UpdateOne(
			context.Background(),
			bson.M{
				"_id":     entryID,
				"user_id": userID,
			},
			bson.M{
				"$set": bson.M{
					"date":     entry.Date,
					"meal":     entry.Meal,
					"calories": entry.Calories,
				},
			},
		)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to update calorie entry"})
			return
		}

		if result.MatchedCount == 0 {
			c.JSON(404, gin.H{"error": "Entry not found or not owned by user"})
			return
		}

		c.JSON(200, gin.H{"message": "Calorie entry updated"})
	}
}