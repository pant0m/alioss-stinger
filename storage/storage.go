package storage

import "fmt"

// Storage 定义所有云厂商对象存储适配器需要实现的接口。
// 所有实现都必须是 goroutine-safe 的。
type Storage interface {
	Put(key string, data []byte) error
	Get(key string) ([]byte, error)
	Delete(key string) error
	List(prefix string, maxKeys int) ([]string, error)
}

// Config 是所有 provider 共用的连接参数。
// 各 provider 对字段的解释略有差异，参见下方常量说明。
type Config struct {
	// Endpoint 对于阿里云是 OSS endpoint（oss-cn-hangzhou.aliyuncs.com），
	// 对于腾讯云 COS / AWS S3 是 region（ap-guangzhou / us-east-1）。
	Endpoint        string
	AccessKeyID     string
	AccessKeySecret string
	Bucket          string
}

const (
	ProviderAliyun  = "aliyun"
	ProviderTencent = "tencent"
	ProviderAWS     = "aws"
)

// New 根据 provider 名称构造对应的 Storage 实例。
func New(provider string, cfg Config) (Storage, error) {
	switch provider {
	case ProviderAliyun:
		return NewAliyun(cfg)
	case ProviderTencent:
		return NewTencent(cfg)
	case ProviderAWS:
		return NewAWS(cfg)
	default:
		return nil, fmt.Errorf("不支持的云厂商: %q (支持: aliyun/tencent/aws)", provider)
	}
}
