package storage

import (
	"bytes"
	"io"

	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
)

type Huawei struct {
	client *obs.ObsClient
	bucket string
}

// NewHuawei 需要 cfg.Endpoint 为 OBS endpoint，例如 obs.cn-north-4.myhuaweicloud.com。
func NewHuawei(cfg Config) (*Huawei, error) {
	client, err := obs.New(cfg.AccessKeyID, cfg.AccessKeySecret, cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	return &Huawei{client: client, bucket: cfg.Bucket}, nil
}

func (h *Huawei) Put(key string, data []byte) error {
	in := &obs.PutObjectInput{}
	in.Bucket = h.bucket
	in.Key = key
	in.Body = bytes.NewReader(data)
	_, err := h.client.PutObject(in)
	return err
}

func (h *Huawei) Get(key string) ([]byte, error) {
	in := &obs.GetObjectInput{}
	in.Bucket = h.bucket
	in.Key = key
	out, err := h.client.GetObject(in)
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (h *Huawei) Delete(key string) error {
	_, err := h.client.DeleteObject(&obs.DeleteObjectInput{
		Bucket: h.bucket,
		Key:    key,
	})
	return err
}

func (h *Huawei) List(prefix string, maxKeys int) ([]string, error) {
	in := &obs.ListObjectsInput{}
	in.Bucket = h.bucket
	in.Prefix = prefix
	in.MaxKeys = maxKeys
	out, err := h.client.ListObjects(in)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(out.Contents))
	for _, o := range out.Contents {
		keys = append(keys, o.Key)
	}
	return keys, nil
}
