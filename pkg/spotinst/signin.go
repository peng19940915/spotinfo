package spotinst

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"net/url"
	"os"
	"spotinfo/pkg/known"
	"time"
)

func Signin(ctx context.Context) error {
	UserName, isExist := os.LookupEnv("SpotinstUsername")
	if !isExist {
		return errors.New("Env: SpotinstUsername not found in system")
	}
	Password, isExist := os.LookupEnv("SpotinstPassword")
	if !isExist {
		return errors.New("env: SpotinstPassword not found in system")
	}
	uri, _ := url.JoinPath(known.SpotHost, known.SpotSignUri)
	req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(resp)
	}()
	req.SetMethod(consts.MethodPost)
	req.SetRequestURI(uri)
	var bodyMap = make(map[string]string, 2)
	bodyMap["email"] = UserName
	bodyMap["password"] = Password
	requestBody, _ := sonic.Marshal(bodyMap)
	req.SetBody(requestBody)
	req.SetHeaders(map[string]string{
		"Accept":          "application/json, text/plain, */*",
		"Accept-Encoding": "gzip, deflate, br",
		"Accept-Language": "en,zh-CN;q=0.9,zh;q=0.8",
		"Content-Type":    "application/json;charset=UTF-8",
		//"Sec-Ch-Ua":       "\"Not/A)Brand\";v=\"99\", \"Google Chrome\";v=\"115\", \"Chromium\";v=\"115\"",
		//"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36",
	})
	hClient, _ := client.NewClient(client.WithTLSConfig(&tls.Config{
		InsecureSkipVerify: true,
	}))

	err := hClient.DoTimeout(ctx, req, resp, 6*time.Second)
	if err != nil {
		return err
	}
	//094fc12e65a1a9b3ead58e232b5b7c91c0f1e9e45a9760412c422cfff581d81d
	rootNode, err := sonic.Get(resp.Body())
	if err != nil {
		errMsg, _ := rootNode.String()
		return errors.New(errMsg)
	}
	fmt.Println(string(resp.Body()))
	known.SPotinstUserToken, err = rootNode.GetByPath("items", 0, "accessToken").String()
	if err != nil {
		return err
	}
	return nil
}
