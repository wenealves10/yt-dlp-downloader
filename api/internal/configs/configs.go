package configs

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	// Ambiente
	Env string `mapstructure:"ENV"`

	// Redis
	RedisHost     string `mapstructure:"REDIS_HOST"`
	RedisPort     string `mapstructure:"REDIS_PORT"`
	RedisUsername string `mapstructure:"REDIS_USERNAME"`
	RedisPassword string `mapstructure:"REDIS_PASSWORD"`

	// AWS S3
	AccountID       string `mapstructure:"ACCOUNT_ID"`
	AccessKeyID     string `mapstructure:"ACCESS_KEY_ID"`
	SecretAccessKey string `mapstructure:"SECRET_ACCESS_KEY"`
	Region          string `mapstructure:"REGION"`
	BucketName      string `mapstructure:"BUCKET_NAME"`
	EndpointURL     string `mapstructure:"ENDPOINT_URL"`

	// Database
	DBHost     string `mapstructure:"DB_HOST"`
	DBPort     string `mapstructure:"DB_PORT"`
	DBUser     string `mapstructure:"DB_USER"`
	DBPassword string `mapstructure:"DB_PASSWORD"`
	DBName     string `mapstructure:"DB_NAME"`

	// Geral
	DBDriver            string        `mapstructure:"DB_DRIVER"`
	DBSource            string        `mapstructure:"DB_SOURCE"`
	ServerAddress       string        `mapstructure:"SERVER_ADDRESS"`
	TokenPasetoKey      string        `mapstructure:"TOKEN_PASETO_KEY"`
	AccessTokenDuration time.Duration `mapstructure:"ACCESS_TOKEN_DURATION"`

	// # Proxy configuration
	ProxyEnabled bool   `mapstructure:"PROXY_ENABLED"`
	ProxyURL     string `mapstructure:"PROXY_URL"`

	// Turnstile
	TurnstileSecret string `mapstructure:"TURNSTILE_SECRET"`

	// YouTube-DL configuration
	YoutubeDLFileCookies string `mapstructure:"YOUTUBE_DL_FILE_COOKIES"`
	YoutubeDLUserAgent   string `mapstructure:"YOUTUBE_DL_USER_AGENT"`
	YoutubeDLReferer     string `mapstructure:"YOUTUBE_DL_REFERER"`
	YoutubeDLAddHeader   string `mapstructure:"YOUTUBE_DL_ADD_HEADER"`
}

var LoadedConfig Config

func LoadConfig(path string) (Config, error) {
	var config Config

	viper.AddConfigPath(path)
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	if _, err := os.Stat(path + "/.env"); err == nil {
		err = viper.ReadInConfig()
		if err != nil {
			return Config{}, fmt.Errorf("erro ao ler .env: %w", err)
		}
	} else {
		fmt.Println("⚠️ Arquivo .env não encontrado, usando apenas variáveis de ambiente")
		bindEnvVariables(&config)
	}

	err := viper.Unmarshal(&config)
	if err != nil {
		return Config{}, fmt.Errorf("erro ao mapear config: %w", err)
	}

	LoadedConfig = config
	return config, nil
}

func bindEnvVariables(cfg interface{}) {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	val := reflect.ValueOf(cfg).Elem()
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag == "" {
			continue
		}
		_ = viper.BindEnv(tag)
	}
}
