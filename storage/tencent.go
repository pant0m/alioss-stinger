package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/tencentyun/cos-go-sdk-v5"
)

type Tencent struct {
	client *cos.Client
}

// NewTencent 需要 cfg.Endpoint 为腾讯云 region，例如 ap-guangzhou；
// cfg.Bucket 必须是完整的 <name>-<appid> 格式，例如 mybucket-1250000000。
func NewTencent(cfg Config) (*Tencent, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("tencent: 需要在 endpoint 字段填入 region，例如 ap-guangzhou")
	}
	u, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cfg.Bucket, cfg.Endpoint))
	if err != nil {
		return nil, err
	}
	baseURL := &cos.BaseURL{BucketURL: u}
	client := cos.NewClient(baseURL, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.AccessKeyID,
			SecretKey: cfg.AccessKeySecret,
		},
	})
	return &Tencent{client: client}, nil
}

func (t *Tencent) Put(key string, data []byte) error {
	_, err := t.client.Object.Put(context.Background(), key, bytes.NewReader(data), nil)
	return err
}

func (t *Tencent) Get(key string) ([]byte, error) {
	resp, err := t.client.Object.Get(context.Background(), key, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (t *Tencent) Delete(key string) error {
	_, err := t.client.Object.Delete(context.Background(), key)
	return err
}

func (t *Tencent) List(prefix string, maxKeys int) ([]string, error) {
	opt := &cos.BucketGetOptions{
		Prefix:  prefix,
		MaxKeys: maxKeys,
	}
	res, _, err := t.client.Bucket.Get(context.Background(), opt)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(res.Contents))
	for _, o := range res.Contents {
		keys = append(keys, o.Key)
	}
	return keys, nil
}
