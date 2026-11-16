package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

type Buckets struct {
	Name      string
	Owner     string
	CreatedAt time.Time
}

type Objects struct {
	Bucket         string
	Key            string
	Size           int64
	ETag           string
	ChecksumSHA256 string
	ContentType    string
	StoragePath    string
	UpdatedAt      time.Time
}

func CreateBucket(ctx context.Context, coll *mongo.Collection, bucket Buckets) (*mongo.InsertOneResult, error) {
	bucket.CreatedAt = time.Now()
	result, err := coll.InsertOne(ctx, bucket)
	if err != nil {
		return nil, err
	}
	return result, nil
}
