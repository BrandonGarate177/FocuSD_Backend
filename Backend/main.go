package main

// Import statements to include packages
import (
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"log"
	"os"

	"yourmodule/handlers"
)

// Main Func always fire to see
func main() {
	_ = godotenv.Load() // Fancy syntax to load .env file
	bucketName := os.Getenv("GCS_BUCKET")

	// If bucket empty ERROR CHECKING
	if bucketName == "" {
		log.Fatal("GCS_BUCKET env var is required")
	}

	// Gin router
	r := gin.Default()

	// ONE endpoint that handles file uploads
	r.POST("/upload", handlers.UploadHandler(bucketName))

	// Starts on port 8080
	r.Run(":8080")
}
