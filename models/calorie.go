package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type CalorieEntry struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID   primitive.ObjectID `bson:"user_id" json:"userId"`
	Date     string             `bson:"date" json:"date"`
	Meal     string             `bson:"meal" json:"meal"`
	Calories int                `bson:"calories" json:"calories"`
}
