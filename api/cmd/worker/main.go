package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
	"github.com/wenealves10/yt-dlp-downloader/internal/configs"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/integrations"
	"github.com/wenealves10/yt-dlp-downloader/internal/jobs"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/storage/r2"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/stream"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
	"github.com/wenealves10/yt-dlp-downloader/internal/providers"
	"github.com/wenealves10/yt-dlp-downloader/internal/queues"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
	"github.com/wenealves10/yt-dlp-downloader/internal/ytaccounts"
)

func main() {
	cg, err := configs.LoadConfig(".")
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	accessKeyId := cg.AccessKeyID
	accessKeySecret := cg.SecretAccessKey
	region := cg.Region
	endpoint := cg.EndpointURL
	bucketName := cg.BucketName

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyId, accessKeySecret, "")),
		config.WithRegion(region),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create an S3 client with the custom endpoint for Cloudflare R2
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})
	r2Storage := r2.NewS3Service(s3Client, bucketName)

	// Initialize the Database connection
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cg.DBSource)
	if err != nil {
		log.Fatal("Erro ao conectar no banco:", err)
	}
	defer pool.Close()
	store := db.NewStore(pool)

	redisAddr := fmt.Sprintf("%s:%s", cg.RedisHost, cg.RedisPort)
	// Initialize the Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Username: cg.RedisUsername,
		Password: cg.RedisPassword,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("could not connect to Redis: %v", err)
	}
	defer rdb.Close()

	rdStream := stream.NewRedisPublisher(rdb)

	asynqRedisOpt := asynq.RedisClientOpt{
		Addr:     redisAddr,
		Username: cg.RedisUsername,
		Password: cg.RedisPassword,
	}

	browserClient := browser.NewClient(cg.BrowserServiceURL, cg.BrowserServiceToken, cg.BrowserServiceTimeout)
	accountProvider := ytaccounts.NewManager(store, browserClient)
	mediaRegistry := providers.Registry(cg, media.RoleDownloader)
	// O painel roda na API; é daqui que sai o diagnóstico do processo que
	// realmente baixa.
	saudeMidia := jobs.NewJobMediaHealth(rdb, mediaRegistry, media.RoleDownloader)

	srv := asynq.NewServer(
		asynqRedisOpt,
		asynq.Config{
			Concurrency: queues.WorkerConcurrency,
			Queues: map[string]int{
				queues.TypeDownloadVideoQueue:      queues.QueueWeightDownloadVideo,
				queues.TypeDownloadMusicQueue:      queues.QueueWeightDownloadMusic,
				queues.TypeUploadVideoQueue:        queues.QueueWeightUploadVideo,
				queues.TypeUploadMusicQueue:        queues.QueueWeightUploadMusic,
				queues.TypeDownloadExpirationQueue: queues.QueueWeightFileExpiration,
				queues.TypeDeleteDownloadQueue:     queues.QueueWeightDeleteDownload,
				queues.TypeYoutubeHealthCheckQueue: queues.QueueWeightYoutubeHealth,
				queues.TypeDownloadMediaQueue:      queues.QueueWeightDownloadMedia,
				queues.TypeTempCleanupQueue:        queues.QueueWeightTempCleanup,
				queues.TypeMediaHealthQueue:        queues.QueueWeightMediaHealth,
				// A entrega de webhook tem fila própria: um endpoint lento de
				// cliente não pode ocupar os slots que deveriam baixar vídeo.
				queues.TypeIntegrationWebhookQueue: queues.QueueWeightIntegrationWebhook,
				queues.TypeIntegrationPruneQueue:   queues.QueueWeightIntegrationPrune,
			},
		},
	)

	// Um container derrubado no meio de um download não roda a limpeza do job;
	// varrer na subida é o que impede o disco encher com sobras de execuções
	// anteriores.
	jobs.LimparTemporariosOrfaos(cg.MediaWorkDir)

	// Publica já na subida: esperar o primeiro agendamento deixaria o painel
	// sem resposta por minutos logo depois de um deploy.
	saudeMidia.Publicar(ctx)

	go startScheduledTasks(asynqRedisOpt, browserClient.Configured())

	asynqClient := asynq.NewClient(asynqRedisOpt)
	defer asynqClient.Close()

	mux := asynq.NewServeMux()
	// O job unificado atende qualquer plataforma. Os dois específicos seguem
	// registrados só para as tarefas que já estavam na fila quando esta versão
	// subiu; downloads novos não passam mais por eles.
	mux.Handle(tasks.TypeDownloadMedia, jobs.NewJobDownloadMedia(
		asynqClient, store, rdStream, rdb, mediaRegistry, accountProvider, cg.MediaWorkDir))
	mux.Handle(tasks.TypeDownloadVideo, jobs.NewJobDownloadVideo(asynqClient, store, rdStream, accountProvider))
	mux.Handle(tasks.TypeDownloadMusic, jobs.NewJobDownloadMusic(asynqClient, store, rdStream, accountProvider))
	mux.Handle(tasks.TypeUploadVideo, jobs.NewJobUploadVideo(asynqClient, r2Storage, store, rdStream))
	mux.Handle(tasks.TypeUploadMusic, jobs.NewJobUploadMusic(asynqClient, r2Storage, store, rdStream))
	mux.Handle(tasks.TypeDeleteDownload, jobs.NewJobDeleteDownload(asynqClient, r2Storage, store))
	mux.Handle(tasks.TypeDownloadExpiration, jobs.NewJobDownloadExpiration(asynqClient, store))
	mux.Handle(tasks.TypeYoutubeHealthCheck, jobs.NewJobYoutubeHealthCheck(store, browserClient))
	mux.Handle(tasks.TypeTempCleanup, jobs.NewJobTempCleanup(cg.MediaWorkDir))

	// Notificações para os sistemas integrados. O entregador é montado uma vez
	// e reaproveitado: ele carrega o pool de conexões e as travas de segurança
	// da saída (sem redirecionamento, sem endereço interno, com timeout).
	entregador := integrations.NewEntregador(cg.WebhookTimeout, cg.WebhookAllowPrivate)
	mux.Handle(tasks.TypeIntegrationWebhook, jobs.NewJobWebhookDeliver(store, entregador))
	mux.Handle(tasks.TypeIntegrationPrune,
		jobs.NewJobIntegrationPrune(store, cg.IntegrationLogRetentionDays))

	mux.Handle(tasks.TypeMediaHealth, saudeMidia)
	log.Printf("Starting worker server on %s with a maximum of %d concurrent tasks", redisAddr, queues.WorkerConcurrency)

	if err := srv.Run(mux); err != nil {
		log.Fatalf("could not run server: %v", err)
	}
}

func startScheduledTasks(redisOpt asynq.RedisClientOpt, withYoutubeHealthCheck bool) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic(err)
	}
	scheduler := asynq.NewScheduler(
		redisOpt,
		&asynq.SchedulerOpts{
			Location: loc,
		},
	)

	task, err := tasks.NewDownloadExpirationTask()
	if err != nil {
		log.Fatalf("failed to create download expiration task: %v", err)
	}

	entryID, err := scheduler.Register("*/5 * * * *", task, asynq.Queue(queues.TypeDownloadExpirationQueue))
	if err != nil {
		log.Fatal(err)
	}

	// O health check das sessões só é agendado quando o serviço de navegador
	// existe, para não gerar ruído em ambientes sem contas gerenciadas.
	if withYoutubeHealthCheck {
		healthTask, err := tasks.NewYoutubeHealthCheckTask()
		if err != nil {
			log.Fatalf("failed to create youtube health check task: %v", err)
		}
		healthEntryID, err := scheduler.Register("*/15 * * * *", healthTask, asynq.Queue(queues.TypeYoutubeHealthCheckQueue))
		if err != nil {
			log.Fatal(err)
		}
		log.Println("🩺 Health check das contas do YouTube agendado...", "Entry ID:", healthEntryID)
	}

	limpezaTask, err := tasks.NewTempCleanupTask()
	if err != nil {
		log.Fatalf("failed to create temp cleanup task: %v", err)
	}
	if _, err := scheduler.Register("17 * * * *", limpezaTask, asynq.Queue(queues.TypeTempCleanupQueue)); err != nil {
		log.Fatal(err)
	}

	// O relatório expira em 10 minutos; publicar a cada 5 mantém o painel com
	// dado fresco e faz um worker morto aparecer como silencioso.
	saudeTask, err := tasks.NewMediaHealthTask()
	if err != nil {
		log.Fatalf("failed to create media health task: %v", err)
	}
	if _, err := scheduler.Register("@every 5m", saudeTask, asynq.Queue(queues.TypeMediaHealthQueue)); err != nil {
		log.Fatal(err)
	}

	// Poda da auditoria das integrações. De madrugada, e uma vez por dia: é
	// DELETE em duas tabelas que podem ser grandes, e no horário de pico isso
	// competiria com as consultas do painel.
	podaTask, err := tasks.NewIntegrationPruneTask()
	if err != nil {
		log.Fatalf("failed to create integration prune task: %v", err)
	}
	if _, err := scheduler.Register("23 4 * * *", podaTask,
		asynq.Queue(queues.TypeIntegrationPruneQueue)); err != nil {
		log.Fatal(err)
	}

	log.Println("⏱️ Scheduler iniciado...", "Entry ID:", entryID)
	if err := scheduler.Run(); err != nil {
		log.Fatalf("could not run scheduler: %v", err)
	}
}
