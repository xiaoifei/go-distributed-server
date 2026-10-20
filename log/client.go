package log

import (
	"bytes"
	"fmt"
	stdlog "log"
	"net/http"

	"xiaoifei.top/registry"
)

// 提供给其他服务，log的使用方法
func SetClientLogger(serviceURL string, clientService registry.ServiceName) {
	stdlog.SetPrefix(fmt.Sprintf("[%v] - ", clientService))
	stdlog.SetFlags(0)
	stdlog.SetOutput(&clientLogger{url: serviceURL})
}

type clientLogger struct {
	url string
}

func (this clientLogger) Write(p []byte) (n int, err error) {
	b := bytes.NewBuffer(p)
	res, err := http.Post(this.url+"log", "text/plain", b)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Faild to send log message. Service responded with %v", res.StatusCode)
	}
	return len(p), nil
}
