package server

import (
	"context"
	"net/http"

	"github.com/fehmicorp/s3object/v2/internal/cache"
	"github.com/fehmicorp/s3object/v2/internal/db"
	"github.com/fehmicorp/s3object/v2/internal/storage"
)

type Buckets struct {
	Ctx     *context.Context
	DB      *db.DB
	Cache   *cache.Cache
	Storage *storage.Type
	Params  Params
}

func GetListBuckets() (*http.Response, error) {
	data := map[string]any{"action": "list_buckets"}
	resp, err := ResponseWriter(http.StatusOK, data)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (b Buckets) CreateBucket() (*http.Response, error) {
	data := map[string]any{
		"action": "create_bucket",
		"params": b.Params.Keys,
	}

	resp, err := ResponseWriter(http.StatusOK, data)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (p Params) DeleteBucket() (*http.Response, error) {
	data := map[string]any{
		"action": "delete_bucket",
		"params": p.Keys,
	}
	resp, err := ResponseWriter(http.StatusOK, data)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
