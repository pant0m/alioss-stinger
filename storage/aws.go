package storage

import (
	"bytes"
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type AWS struct {
	client *s3.Client
	bucket string
}

// NewAWS 需要 cfg.Endpoint 为 AWS region，例如 us-east-1。
func NewAWS(cfg Config) (*AWS, error) {
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Endpoint),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.AccessKeySecret, "",
		)),
	)
	if err != nil {
		return nil, err
	}
	return &AWS{
		client: s3.NewFromConfig(awsCfg),
		bucket: cfg.Bucket,
	}, nil
}

func (a *AWS) Put(key string, data []byte) error {
	_, err := a.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(a.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	return err
}

func (a *AWS) Get(key string) ([]byte, error) {
	out, err := a.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(a.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (a *AWS) Delete(key string) error {
	_, err := a.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(a.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (a *AWS) List(prefix string, maxKeys int) ([]string, error) {
	out, err := a.client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket:  aws.String(a.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(int32(maxKeys)),
	})
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(out.Contents))
	for _, o := range out.Contents {
		if o.Key != nil {
			keys = append(keys, *o.Key)
		}
	}
	return keys, nil
}
