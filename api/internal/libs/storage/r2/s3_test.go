package r2

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
)

func TestPresignDownloadSignsR2ObjectAndAttachmentHeaders(t *testing.T) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("access-key", "secret-key", "")),
	)
	require.NoError(t, err)

	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String("https://account-id.r2.cloudflarestorage.com")
	})
	service := NewS3Service(client, "private-downloads")

	presignedURL, err := service.PresignDownload(
		context.Background(),
		"uploads/videos/video.mp4",
		"vídeo final.mp4",
		"video/mp4",
		60*time.Second,
	)
	require.NoError(t, err)

	parsed, err := url.Parse(presignedURL)
	require.NoError(t, err)
	require.Contains(t, parsed.Host, "r2.cloudflarestorage.com")
	require.Equal(t, "60", parsed.Query().Get("X-Amz-Expires"))
	require.NotEmpty(t, parsed.Query().Get("X-Amz-Signature"))
	require.Equal(t, `attachment; filename*=utf-8''v%C3%ADdeo%20final.mp4`, parsed.Query().Get("response-content-disposition"))
	require.Equal(t, "video/mp4", parsed.Query().Get("response-content-type"))
}
