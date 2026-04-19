package storage

import (
	"bytes"
	"io"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type Aliyun struct {
	bucket *oss.Bucket
}

func NewAliyun(cfg Config) (*Aliyun, error) {
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return nil, err
	}
	b, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, err
	}
	return &Aliyun{bucket: b}, nil
}

func (a *Aliyun) Put(key string, data []byte) error {
	return a.bucket.PutObject(key, bytes.NewReader(data))
}

func (a *Aliyun) Get(key string) ([]byte, error) {
	r, err := a.bucket.GetObject(key)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (a *Aliyun) Delete(key string) error {
	return a.bucket.DeleteObject(key)
}

func (a *Aliyun) List(prefix string, maxKeys int) ([]string, error) {
	res, err := a.bucket.ListObjects(oss.MaxKeys(maxKeys), oss.Prefix(prefix))
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(res.Objects))
	for _, o := range res.Objects {
		keys = append(keys, o.Key)
	}
	return keys, nil
}
