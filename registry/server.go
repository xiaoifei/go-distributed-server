package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const ServerPort = ":3000"
const ServicesURL = "http://localhost" + ServerPort + "/services"

type registry struct {
	registrations []Registration
	mutex         *sync.RWMutex
}

func (r *registry) add(reg Registration) error {
	// 向刚注册服务的发送已注册的依赖服务的信息
	if err := r.sendRequireServices(reg); err != nil {
		return err
	}
	r.mutex.Lock()
	r.registrations = append(r.registrations, reg)
	r.mutex.Unlock()
	// 通知依赖该服务的服务进行更新
	r.notify(patch{
		Added: []patchEntry{
			{
				Name: reg.ServiceName,
				URL:  reg.ServiceURL,
			},
		},
	})
	return nil
}

func (r *registry) notify(commingPatch patch) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, reg := range r.registrations {
		go func(reg Registration) {
			for _, reqService := range reg.RequiredServices {
				p := patch{Added: []patchEntry{}, Removed: []patchEntry{}}
				sendUpdate := false
				for _, added := range commingPatch.Added {
					if added.Name == reqService {
						p.Added = append(p.Added, added)
						sendUpdate = true
					}
				}
				for _, removed := range commingPatch.Removed {
					if removed.Name == reqService {
						p.Removed = append(p.Removed, removed)
						sendUpdate = true
					}
				}
				if sendUpdate {
					// TODO 可能会失败
					err := r.sendPatch(reg.ServiceHookURL, p)
					if err != nil {
						log.Println(err)
						return
					}
				}
			}
		}(reg)
	}
}

func (r *registry) sendRequireServices(reg Registration) error {
	r.mutex.RLock()
	var p patch
	for _, reqService := range reg.RequiredServices { // 遍历依赖的服务
		for _, exService := range r.registrations { // 遍历当前已经注册的服务
			if reqService == exService.ServiceName {
				// 将已注册的服务信息发给待注册服务
				p.Added = append(p.Added, patchEntry{exService.ServiceName, exService.ServiceURL})
			}
		}
	}
	r.mutex.RUnlock()
	if len(p.Added) == 0 { // 没有依赖在册，就别发空 patch 了
		return nil
	}
	return r.sendPatch(reg.ServiceHookURL, p)
}

func (r *registry) sendPatch(url string, p patch) error {
	var buf *bytes.Buffer = new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("hook %s responded with %d", url, res.StatusCode)
	}
	return nil
}

// 通知依赖该服务的服务
func (r *registry) remove(url string) error {
	r.mutex.Lock()
	var removed patch
	for i := range r.registrations {
		if r.registrations[i].ServiceURL == url {
			removed = patch{
				Removed: []patchEntry{{
					Name: r.registrations[i].ServiceName,
					URL:  r.registrations[i].ServiceURL,
				}},
			}
			r.registrations = append(r.registrations[:i], r.registrations[i+1:]...)
			r.mutex.Unlock() // 先解锁避免死锁
			r.notify(removed)
			return nil
		}
	}
	r.mutex.Unlock()
	return fmt.Errorf("Failed to find the service to be removed with URL : %v", url)
}

func (r *registry) heartbeat(freq time.Duration) {
	for {
		var wg sync.WaitGroup
		for _, reg := range r.registrations {
			wg.Add(1)
			go func(reg Registration) {
				defer wg.Done()
				success := true
				for attemps := 0; attemps < 3; attemps++ {
					res, err := http.Get(reg.HeartbeatURL)
					if err != nil {
						log.Println(err)
					} else if res.StatusCode == http.StatusOK {
						log.Printf("Heartbeat check pass for %v", reg.ServiceName)
						if !success {
							r.add(reg)
						}
						break
					}
					if success {
						success = false
						r.remove(reg.ServiceURL)
					}
					time.Sleep(1 * time.Second)
				}
			}(reg)
		}
		wg.Wait()
		time.Sleep(freq)
	}
}

var once sync.Once

func SetupRegistryService() {
	once.Do(func() {
		go reg.heartbeat(3 * time.Second)
	})
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
