package middlewares

import (
	"time"

	"chango/config"

	"github.com/dgrijalva/jwt-go"
)

func GenerateToken(username string, uniqueKey string) (string, interface{}, error) {
	// Create the token
	k := []byte(config.LoadEnv("SIGNED_KEY"))
	token := jwt.New(jwt.SigningMethodHS256)
	// Set some claims
	claims := make(jwt.MapClaims)
	claims["username"] = username
	claims["exp"] = time.Now().Add(time.Hour * 72).Unix()
	claims["unique_key"] = uniqueKey
	token.Claims = claims
	// Sign and get the complete encoded token as a string
	tokenString, err := token.SignedString(k)
	return tokenString, claims["exp"], err
}

func ValidateToken(t string, k string) (*jwt.Token, error) {
	token, err := jwt.Parse(t, func(token *jwt.Token) (interface{}, error) {
		return []byte(k), nil
	})

	return token, err
}
