package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

var validKeyHash []byte

// loadKeyHash reads the hash from the text file once when the program starts
func loadKeyHash() {
	// Read the hash string from file
	data := os.Getenv("API_KEY_HASH")
	//fmt.Println(data)
	/*
		data, err := os.ReadFile("valid_key_hash.txt")
		if err != nil {
			log.Fatal("Could not read valid_key_hash.txt: ", err)
		}
	*/
	// Clean up whitespace/newlines
	cleanHex := strings.TrimSpace(data)

	// Decode hex string back to bytes for comparison
	decoded, err := hex.DecodeString(cleanHex)
	if err != nil {
		log.Fatal("Invalid hex in text file: ", err)
	}
	validKeyHash = decoded
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// 1. Get the raw key from the user
		incomingKey := strings.TrimPrefix(authHeader, "Bearer ")

		// 2. Hash the incoming key
		hash := sha256.Sum256([]byte(incomingKey))

		// 3. Compare the calculated hash with the stored hash
		// Use subtle.ConstantTimeCompare to prevent timing attacks
		if subtle.ConstantTimeCompare(hash[:], validKeyHash) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API Key"})
			c.Abort()
			return
		}

		c.Next()
	}
}
