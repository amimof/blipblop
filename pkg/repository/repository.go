package repository

import (
	"context"
)

type Repository interface {
	Create(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
	GetAll(context.Context) ([][]byte, error)
	Update(context.Context, string, []byte) error
}
