package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"yourmodule/services"
)

// Like the views.py in Django. Prossessing file uploads in Go with Gin is done through handlers.
func UploadHandler(bucketName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, header, err := c.Request.FormFile("image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No image uploaded"})
			return
		}
		defer file.Close()

		filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), header.Filename)
		localPath := filepath.Join("uploads", filename)

		os.MkdirAll("uploads", 0755)
		out, err := os.Create(localPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
			return
		}
		_, err = io.Copy(out, file)
		out.Close()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
			return
		}

		gcsURL, err := services.UploadToGCS(c, bucketName, filename, localPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload to GCS"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"url": gcsURL})
	}
}
