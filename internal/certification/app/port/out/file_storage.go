package out

import (
	"context"
	"io"
)

// FileStorage keeps attachment bytes behind the hexagon: the MVP adapter is a
// Docker-volume disk; a MinIO/S3 adapter is a composition-root swap later.
type FileStorage interface {
	Put(ctx context.Context, key string, content io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}
