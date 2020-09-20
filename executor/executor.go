package executor

import (
	"context"
	"io"
)

type Session interface {
	Reader() io.Reader
	Writer() io.Writer
	CloseWrite() error
	End(ctx context.Context) (int, error)
}

type Executor interface {
	Name() string
	Session(ctx context.Context) (Session, error)
	Close(ctx context.Context) error
}
