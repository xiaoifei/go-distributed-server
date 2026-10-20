package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
)

const ServerPort = ":3000"
const ServicesURL = "http://localhost" + ServerPort + "/services"

type registry struct {
	registrations []Registration
	mutex         *sync.RWMutex
}

func (r *registry) add(reg Registration) error {
	r.mutex.Lock()
	r.registrations = append(r.registrations, reg)
	r.mutex.Unlock()
	// 向刚注册服务的发送已注册的依赖服务的信息
	err := r.sendRequireServices(reg)
	if err != nil {
		return err
	}
	return nil
}

func (r *registry) sendRequireServices(reg Registration) error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var p patch
	for _, reqService := range reg.RequiredServices { // 遍历依赖的服务
		for _, exService := range r.registrations { // 遍历当前已经注册的服务
			if reqService == exService.ServiceName {
				// 将已注册的服务信息发给待注册服务
				p.Added = append(p.Added, patchEntry{exService.ServiceName, exService.ServiceURL})
				err := r.sendPatch(reg.ServiceHookURL, p)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (r *registry) sendPatch(url string, p patch) error {
	var buf *bytes.Buffer = new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, buf)
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	return nil
}

func (r *registry) remove(url string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for i := range reg.registrations {
		if reg.registrations[i].ServiceURL == url {
			reg.registrations = append(reg.registrations[:i], reg.registrations[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("Failed to find the service to be removed with URL : %v", url)
}

var reg = registry{
	registrations: make([]Registration, 0),
	mutex:         new(sync.RWMutex),
}

type RegistryService struct{}

func (s RegistryService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Request received")
	switch r.Method {
	case http.MethodPost:
		dec := json.NewDecoder(r.Body)
		var r Registration
		if err := dec.Decode(&r); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("Adding service: %v with URL: %v\n", r.ServiceName, r.ServiceURL)

		if err := reg.add(r); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	case http.MethodDelete:
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		url := string(payload)
		log.Printf("Removing service at URL: %s", url)
		err = reg.remove(url)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
