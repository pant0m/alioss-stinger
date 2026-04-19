package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	qstorage "github.com/qiniu/go-sdk/v7/storage"
)

type Qiniu struct {
	mac    *qbox.Mac
	bucket string
	domain string
	bm     *qstorage.BucketManager
	up     *qstorage.FormUploader
}

// NewQiniu 需要 cfg.Domain 为 Kodo 存储空间绑定的下载域名（不含 scheme）。
// cfg.Endpoint 可留空，SDK 会自动选择区域。
func NewQiniu(cfg Config) (*Qiniu, error) {
	if cfg.Domain == "" {
		return nil, fmt.Errorf("qiniu: 需要提供下载域名 (domain)")
	}
	mac := qbox.NewMac(cfg.AccessKeyID, cfg.AccessKeySecret)
	qcfg := &qstorage.Config{}
	return &Qiniu{
		mac:    mac,
		bucket: cfg.Bucket,
		domain: cfg.Domain,
		bm:     qstorage.NewBucketManager(mac, qcfg),
		up:     qstorage.NewFormUploader(qcfg),
	}, nil
}

func (q *Qiniu) Put(key string, data []byte) error {
	policy := qstorage.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", q.bucket, key),
	}
	token := policy.UploadToken(q.mac)
	ret := qstorage.PutRet{}
	return q.up.Put(context.Background(), &ret, token, key, bytes.NewReader(data), int64(len(data)), nil)
}

func (q *Qiniu) Get(key string) ([]byte, error) {
	deadline := time.Now().Add(1 * time.Minute).Unix()
	url := qstorage.MakePrivateURL(q.mac, q.domain, key, deadline)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qiniu get %s: http %d", key, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (q *Qiniu) Delete(key string) error {
	return q.bm.Delete(q.bucket, key)
}

func (q *Qiniu) List(prefix string, maxKeys int) ([]string, error) {
	entries, _, _, _, err := q.bm.ListFiles(q.bucket, prefix, "", "", maxKeys)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		keys = append(keys, e.Key)
	}
	return keys, nil
}
