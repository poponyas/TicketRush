package s3

import (
	"context"
	"github.com/google/uuid"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
	"github.com/stretchr/testify/require"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestPrivateS3RoundTrip(t *testing.T) {
	endpoint := os.Getenv("TICKETRUSH_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set TICKETRUSH_TEST_S3_ENDPOINT for S3 integration tests")
	}
	s, err := New(Config{Endpoint: endpoint, AccessKey: "integration_test", SecretKey: "integration_test_secret", Bucket: "test-" + uuid.NewString(), Timeout: 10 * time.Second})
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, s.EnsureBucket(ctx))
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_ = s.Delete(c, "poster.png")
		require.NoError(t, s.client.RemoveBucket(c, s.bucket))
	})
	data := []byte("test-object-data")
	require.NoError(t, s.Put(ctx, "poster.png", data, "image/png"))
	got, kind, err := s.Get(ctx, "poster.png")
	require.NoError(t, err)
	require.Equal(t, data, got)
	require.Equal(t, "image/png", kind)
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get("http://" + endpoint + "/" + s.bucket + "/poster.png")
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, 403, res.StatusCode, "bucket must not allow anonymous reads")
	require.NoError(t, s.Delete(ctx, "poster.png"))
	_, _, err = s.Get(ctx, "poster.png")
	require.ErrorIs(t, err, coreerrors.ErrNotFound)
}
