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

	// ResolveProxyURL é um proxy usado SÓ para extrair metadados das
	// plataformas que recusam IP de datacenter. Os bytes do vídeo nunca passam
	// por ele.
	//
	// A separação existe por causa da conta: um proxy residencial é vendido por
	// volume, e a extração custa dezenas de KB enquanto o vídeo custa dezenas
	// de MB. Roteando tudo, 1 GB de cota dá cerca de cem vídeos; roteando só a
	// extração, dá dezenas de milhares.
	ResolveProxyURL string `mapstructure:"RESOLVE_PROXY_URL"`

	// Turnstile
	TurnstileSecret string `mapstructure:"TURNSTILE_SECRET"`

	// YouTube-DL configuration
	YoutubeDLFileCookies string `mapstructure:"YOUTUBE_DL_FILE_COOKIES"`
	YoutubeDLUserAgent   string `mapstructure:"YOUTUBE_DL_USER_AGENT"`
	YoutubeDLReferer     string `mapstructure:"YOUTUBE_DL_REFERER"`
	YoutubeDLAddHeader   string `mapstructure:"YOUTUBE_DL_ADD_HEADER"`
	LimitDownloadFree    int64  `mapstructure:"LIMIT_DOWNLOAD_FREE"`
	LimitDownloadPremium int64  `mapstructure:"LIMIT_DOWNLOAD_PREMIUM"`

	// Serviço de navegador remoto (advideo-browser). A URL só é resolvível na
	// rede interna e o token é o segredo compartilhado entre API, worker e o
	// serviço. Nunca deve ser exposto ao frontend.
	BrowserServiceURL     string        `mapstructure:"BROWSER_SERVICE_URL"`
	BrowserServiceToken   string        `mapstructure:"BROWSER_SERVICE_TOKEN"`
	BrowserServiceTimeout time.Duration `mapstructure:"BROWSER_SERVICE_TIMEOUT"`

	// Promove este e-mail a super admin durante o start do servidor. Serve
	// apenas para o bootstrap do primeiro administrador.
	SuperAdminEmail string `mapstructure:"SUPER_ADMIN_EMAIL"`

	// Mecanismo de download multiplataforma. Trocar a versão do yt-dlp ou o
	// caminho dos binários é configuração, não mudança de código.
	YtDlpBinary          string        `mapstructure:"YTDLP_BINARY"`
	FFmpegBinary         string        `mapstructure:"FFMPEG_BINARY"`
	MediaMetadataTimeout time.Duration `mapstructure:"MEDIA_METADATA_TIMEOUT"`
	MediaDownloadTimeout time.Duration `mapstructure:"MEDIA_DOWNLOAD_TIMEOUT"`
	MediaWorkDir         string        `mapstructure:"MEDIA_WORK_DIR"`

	// Configuração exclusiva do serviço de navegador.
	BrowserListenAddress string        `mapstructure:"BROWSER_LISTEN_ADDRESS"`
	BrowserProfilesDir   string        `mapstructure:"BROWSER_PROFILES_DIR"`
	BrowserBinary        string        `mapstructure:"BROWSER_BINARY"`
	BrowserIdleTimeout   time.Duration `mapstructure:"BROWSER_IDLE_TIMEOUT"`
	BrowserScreenSize    string        `mapstructure:"BROWSER_SCREEN_SIZE"`
	BrowserMaxSessions   int           `mapstructure:"BROWSER_MAX_SESSIONS"`
	// Escape hatch para kernels/hosts onde o sandbox do Chrome não sobe. O
	// padrão é manter o sandbox ligado.
	BrowserDisableSandbox bool `mapstructure:"BROWSER_DISABLE_SANDBOX"`
}

var LoadedConfig Config

func LoadConfig(path string) (Config, error) {
	var config Config

	viper.AddConfigPath(path)
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	// Garante que variáveis fornecidas pelo ambiente (por exemplo, pelo
	// compose.dev.yaml) tenham precedência sobre os valores de .env. Assim o
	// desenvolvimento usa banco e Redis locais sem duplicar as credenciais do
	// R2 já presentes no arquivo de configuração.
	bindEnvVariables(&config)

	if _, err := os.Stat(path + "/.env"); err == nil {
		err = viper.ReadInConfig()
		if err != nil {
			return Config{}, fmt.Errorf("erro ao ler .env: %w", err)
		}
	} else {
		fmt.Println("⚠️ Arquivo .env não encontrado, usando apenas variáveis de ambiente")
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
