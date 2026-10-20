package log

import (
	"io"
	stdlog "log"
	"net/http"
	"os"
)

var log *stdlog.Logger

type fileLog string

func (fl fileLog) Write(data []byte) (int, error) {
	f, err := os.OpenFile(string(fl), os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.FileMode(0600))
	if err != nil {
		return 0, nil
	}
	defer f.Close()
	return f.Write(data)
}

func Run(destination string) {
	log = stdlog.New(fileLog(destination), "[go] - ", stdlog.LstdFlags)
}

func RegisterHandelers() {
	http.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			body, err := io.ReadAll(r.Body)
			if err != nil || len(body) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			write(string(body))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func write(content string) {
	log.Printf("%v\n", content)
}
