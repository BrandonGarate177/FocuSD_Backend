package services

import (
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

func UploadToGCS(c context.Context, bucketName, filename, localPath string) (string, error) {

	client, err := storage.NewClient(c)
	if err != nil {
		return "", err
	}
	defer client.Close()

	bucket := client.Bucket(bucketName)
	object := bucket.Object(filename)
	wc := object.NewWriter(c)
	defer wc.Close()

	f, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(wc, f); err != nil {
		return "", err
	}

	if err := wc.Close(); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, filename)
	return url, nil
}
