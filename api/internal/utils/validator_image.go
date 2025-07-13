package utils

import (
	"mime/multipart"
	"net/http"
)

func ValidateImage(file *multipart.FileHeader) bool {
	if file == nil {
		return true
	}

	const maxSize = 10 << 20 // 10 MB
	if file.Size <= 0 || file.Size > maxSize {
		return false
	}

	f, err := file.Open()
	if err != nil {
		return false
	}
	defer f.Close()

	header := make([]byte, 512)
	if _, err := f.Read(header); err != nil {
		return false
	}

	mimeType := http.DetectContentType(header)

	switch mimeType {
	case "image/jpeg", "image/png":
		return true
	default:
		return false
	}
}
