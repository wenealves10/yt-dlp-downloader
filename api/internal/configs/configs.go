package configs

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Variables for the database
var (
	ENV = "development"
)

// Variables for the database
var (
	REDIS_HOST     = ""
	REDIS_PORT     = ""
	REDIS_USERNAME = ""
	REDIS_PASSWORD = ""
)

// Variables for AWS S3
var (
	ACCOUNT_ID        = ""
	ACCESS_KEY_ID     = ""
	SECRET_ACCESS_KEY = ""
	REGION            = ""
	BUCKET_NAME       = ""
	ENDPOINT_URL      = ""
)

// DB variables
var (
	DB_HOST     = ""
	DB_PORT     = ""
	DB_USER     = ""
	DB_PASSWORD = ""
	DB_NAME     = ""
)

func init() {
	ENV = os.Getenv("ENV")

	if ENV != "production" {
		err := godotenv.Load(".env")
		if err != nil {
			fmt.Println(err)
		}
	}

	fmt.Printf("Loading environment variables for %s\n", ENV)

	// AWS S3
	ACCESS_KEY_ID = os.Getenv("ACCESS_KEY_ID")
	SECRET_ACCESS_KEY = os.Getenv("SECRET_ACCESS_KEY")
	REGION = os.Getenv("REGION")
	BUCKET_NAME = os.Getenv("BUCKET_NAME")
	ENDPOINT_URL = os.Getenv("ENDPOINT_URL")
	ACCOUNT_ID = os.Getenv("ACCOUNT_ID")

	// REDIS
	REDIS_HOST = os.Getenv("REDIS_HOST")
	REDIS_USERNAME = os.Getenv("REDIS_USERNAME")
	REDIS_PASSWORD = os.Getenv("REDIS_PASSWORD")
	REDIS_PORT = os.Getenv("REDIS_PORT")

	// DB
	DB_HOST = os.Getenv("DB_HOST")
	DB_PORT = os.Getenv("DB_PORT")
	DB_USER = os.Getenv("DB_USER")
	DB_PASSWORD = os.Getenv("DB_PASSWORD")
	DB_NAME = os.Getenv("DB_NAME")

}
