package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Like the views.py in Django. Processing file uploads in Go with Gin is done through handlers.
func UploadHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Save uploaded image
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

		// Call Python script for ML classification
		pythonPath := os.Getenv("PYTHON_PATH")
		if pythonPath == "" {
			pythonPath = "python3" // Default fallback
		}

		cmd := exec.Command(pythonPath, "model/PythonEyeDetection.py", localPath)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()

		// Delete the image after processing (regardless of ML result)
		defer os.Remove(localPath)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "ML classification failed",
				"details": err.Error(),
				"stderr":  stderr.String(),
			})
			return
		}

		output := stdout.Bytes()
		var mlResult map[string]interface{}
		if err := json.Unmarshal(output, &mlResult); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to parse ML result",
				"details": err.Error(),
				"output":  string(output),
			})
			return
		}

		// Return only ML result
		c.JSON(http.StatusOK, gin.H{"ml_result": mlResult})
	}
}
