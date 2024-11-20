package entity

import (
	"github.com/dgrijalva/jwt-go"
	"time"
)

var JwtKey = []byte("lwh666")

// JWT Claim
type Claims struct {
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"exp"`
	jwt.StandardClaims
}
