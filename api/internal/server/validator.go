package server

import (
	"net/url"
	"strings"

	"github.com/go-playground/validator/v10"
)

var ValidYouTubeURL validator.Func = func(fl validator.FieldLevel) bool {
	if rawURL, ok := fl.Field().Interface().(string); ok {
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			return false
		}

		host := strings.ToLower(parsedURL.Hostname())
		switch host {
		case "youtube.com", "www.youtube.com", "m.youtube.com", "youtu.be":
			return true
		default:
			return false
		}
	}
	return false
}
