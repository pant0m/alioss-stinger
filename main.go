package main

import "C"
import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
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

// var c *oss.Bucket

func InitClient(endPoint, accessKeyId, accessKeySecret, bucketName string) error {
	var ossClient *oss.Client
	var err error

	ossClient, err = oss.New(endPoint, accessKeyId, accessKeySecret)
	if err != nil {
		return err
	}

	var ossBucket *oss.Bucket
	ossBucket, err = ossClient.Bucket(bucketName)
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

var server_address string
var bind_address string

func main() {

	var osskey = flag.String("osskey", "", "format: endpoint:accessKeyId:accessKeySecret:bucketName")
	var mode = flag.String("mode", "", "client/server 二选一")
	var address = flag.String("address", "", "监听地址或者目标地址，格式：127.0.0.1:8080")
	// var proxy = flag.String("proxy", "", "代理服务器上网:http://127.0.0.1:8080,如果有密码:http://x.x.x.x,user,pass")
	flag.Parse()

	timeout = 30
	server_address = *address
	bind_address = *address

	str1 := strings.Split(*osskey, ":")
	// str2 := strings.Split(*proxy, ",")
	// fmt.Println(str2)

	if *mode == "" || *osskey == "" {
		flag.PrintDefaults()
		os.Exit(0)
	}

	InitClient(str1[0], str1[1], str1[2], str1[3])

	if *mode == "client" {
		startClient()
	} else if *mode == "server" {
		startServer()
	}
}

func startServer() {
	log.Println("[+]", "服务端启动成功")
	for {

		time.Sleep(1 * time.Second)
		for _, c2 := range List(Service) {
			if strings.Contains(c2.Key, "client.txt") {
				go process_server(c2.Key)
			}
		}
	}
}
func List(c *Client) []oss.ObjectProperties {

	lsRes, err := c.Bucket.ListObjects(oss.MaxKeys(3), oss.Prefix(""))
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(-1)
	}
	// fmt.Println(lsRes)
	return lsRes.Objects

}

var timeout int

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

func process(conn net.Conn) {
	uuid := uuid.New()
	key := uuid.String()
	defer conn.Close() // 关闭连接
	var buffer bytes.Buffer
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		var buf [1]byte
		n, err := conn.Read(buf[:])
		if err != nil {
			log.Println("[-]", uuid, "read from connect failed, err：", err)
			break
		}
		buffer.Write(buf[:n])
		if strings.Contains(buffer.String(), "\r\n\r\n") {
			//fmt.Println("\n---------DEBUG CLIENT------\n", buffer.String(), "\n----------------------")
			if strings.Contains(buffer.String(), "Content-Length") {

				ContentLength := buffer.String()[strings.Index(buffer.String(), "Content-Length: ")+len("Content-Length: ") : strings.Index(buffer.String(), "Content-Length: ")+strings.Index(buffer.String()[strings.Index(buffer.String(), "Content-Length: "):], "\n")]
				log.Println("[+]", uuid, "数据包长度为：", strings.TrimSpace(ContentLength))
				if strings.TrimSpace(ContentLength) != "0" {
					intContentLength, err := strconv.Atoi(strings.TrimSpace(ContentLength))
					if err != nil {
						log.Println("[-]", uuid, "Content-Length转换失败")
					}

					for i := 1; i <= intContentLength; i++ {
						var b [1]byte
						n, err = conn.Read(b[:])
						if err != nil {
							log.Println("[-]", uuid, "read from connect failed, err", err)
							break
						}
						buffer.Write(b[:n])
					}

				}
			}
			if strings.Contains(buffer.String(), "Transfer-Encoding: chunked") {
				for {
					var b [1]byte
					n, err = conn.Read(b[:])
					if err != nil {
						log.Println("[-]", uuid, "read from connect failed, err", err)
						break
					}
					buffer.Write(b[:n])
					if strings.Contains(buffer.String(), "0\r\n\r\n") {
						break
					}
				}
			}
			log.Println("[+]", uuid, "从客户端接受HTTP头完毕")
			break
		}
	}
	b64 := base64.StdEncoding.EncodeToString(buffer.Bytes())
	Send(Service, key+"/client.txt", b64)
	i := 1
	for {
		i++
		time.Sleep(1 * time.Second)
		if i >= timeout {
			log.Println("[x]", "超时，断开")
			Del(Service, key+"/client.txt")
			return
		}
		buff := Get(Service, key+"/server.txt")
		if buff != nil {
			log.Println("[x]", uuid, "收到服务器消息")
			//fmt.Println(buff)
			Del(Service, key+"/server.txt")
			sDec, err := base64.StdEncoding.DecodeString(string(buff))
			//fmt.Println(sDec)
			if err != nil {
				log.Println("[x]", uuid, "Base64解码错误")
				return
			}
			conn.Write(sDec)
			break
		}
	}
	log.Println("[+]", "发送完成")
}

// // var name string = "1111.txt"
// // var content string = "测试数据"

func Send(c *Client, name string, content string) {

	// 1.通过字符串上传对象
	f := strings.NewReader(content)
	// var err error
	err := c.Bucket.PutObject(name, f)
	if err != nil {
		log.Println("[-]", "上传失败")
		return
	}

}
func Get(c *Client, name string) []byte {

	println(name)
	body, err := c.Bucket.GetObject(name)
	if err != nil {
		return nil
	}
	// 数据读取完成后，获取的流必须关闭，否则会造成连接泄漏，导致请求无连接可用，程序无法正常工作。
	defer body.Close()
	// println(body)
	data, err := ioutil.ReadAll(body)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(-1)
	}
	// fmt.Println(data)
	return data
}

func Del(c *Client, name string) {
	err := c.Bucket.DeleteObject(name)
	if err != nil {
		panic(err)
	}

}

func process_server(name string) {

	uuid := name[:strings.Index(name, "/")]
	log.Println("[+]", "发现客户端："+uuid)
	buff := Get(Service, name)
	sDec, err := base64.StdEncoding.DecodeString(string(buff))
	Del(Service, name)
	conn, err := net.Dial("tcp", server_address)

	if err != nil {
		log.Println("[-]", uuid, "连接CS服务器失败")
		return
	}
	defer conn.Close()
	_, err = conn.Write(sDec)
	if err != nil {
		log.Println("[-]", uuid, "无法向CS服务器发送数据包")
		return
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var buffer bytes.Buffer
	for {
		var buf [1]byte
		n, err := conn.Read(buf[:])
		if err != nil {
			log.Println("[-]", uuid, "read from connect failed, err", err)
			break
		}
		buffer.Write(buf[:n])

		if strings.Contains(buffer.String(), "\r\n\r\n") {
			//fmt.Println("\n---------DEBUG SERVER------", buffer.String(), "\n----------------------")
			if strings.Contains(buffer.String(), "Content-Length") {
				ContentLength := buffer.String()[strings.Index(buffer.String(), "Content-Length: ")+len("Content-Length: ") : strings.Index(buffer.String(), "Content-Length: ")+strings.Index(buffer.String()[strings.Index(buffer.String(), "Content-Length: "):], "\n")]
				log.Println("[+]", uuid, "数据包长度为：", strings.TrimSpace(ContentLength))
				if strings.TrimSpace(ContentLength) != "0" {
					intContentLength, err := strconv.Atoi(strings.TrimSpace(ContentLength))
					if err != nil {
						log.Println("[-]", uuid, "Content-Length转换失败")
					}

					for i := 1; i <= intContentLength; i++ {
						var b [1]byte
						n, err = conn.Read(b[:])
						if err != nil {
							log.Println("[-]", uuid, "read from connect failed, err", err)
							break
						}
						buffer.Write(b[:n])
					}

				}
			}
			if strings.Contains(buffer.String(), "Transfer-Encoding: chunked") {
				for {
					var b [1]byte
					n, err = conn.Read(b[:])
					if err != nil {
						log.Println("[-]", uuid, "read from connect failed, err", err)
						break
					}
					buffer.Write(b[:n])
					if strings.Contains(buffer.String(), "0\r\n\r\n") {
						break
					}
				}
			}
			log.Println("[+]", uuid, "从CS服务器接受完毕")
			break
		}
	}

	b64 := base64.StdEncoding.EncodeToString(buffer.Bytes())
	Send(Service, uuid+"/server.txt", b64)
	log.Println("[+]", uuid, "服务器数据发送完毕")
	return
}
