package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type User struct {
	ID       primitive.ObjectID `bson:"_id,omitempty"`
	GoogleID string             `bson:"google_id,omitempty"`
	Email    string             `bson:"email"`
	Password string             `bson:"password,omitempty"`
	Name     string             `bson:"name,omitempty"`
}
