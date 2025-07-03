package main

// Import statements to include packages
import (
	"Backend/handlers"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"log"
)

// Main Func always fire to see
func main() {
	_ = godotenv.Load() // Fancy syntax to load .env file

	// Gin router
	r := gin.Default()

	// ONE endpoint that handles file uploads
	r.POST("/upload", handlers.UploadHandler())
	r.POST("/analyze", handlers.AnalyzeHandler())

	// Starts on port 8080
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
