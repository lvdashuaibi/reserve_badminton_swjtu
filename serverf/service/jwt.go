package service

import (
	"github.com/dgrijalva/jwt-go"
	"serverf/entity"
	"time"
)

func GenerateJWT(username string) (string, error) {
	expirationTime := time.Now().Add(30 * 24 * time.Hour)
	claims := &entity.Claims{
		Username:  username,
		ExpiresAt: expirationTime,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(entity.JwtKey)
}
