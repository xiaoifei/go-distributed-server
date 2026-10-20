package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
)

func RegisterService(r Registration) error {
	serviceHookURL, err := url.Parse(r.ServiceHookURL)
	if err != nil {
		return err
	}
	http.Handle(serviceHookURL.Path, &serviceHookHandler{})

	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	err = enc.Encode(r)
	if err != nil {
		return err
	}

	res, err := http.Post(ServicesURL, "application/json", buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to register service. Responded with code %v", res.StatusCode)
	}

	return nil
}
func ShutdownService(url string) error {
	buf := bytes.NewBufferString(url)
	req, err := http.NewRequest("DELETE", ServicesURL, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to remove service. Responded with code %v", res.StatusCode)
	}

	return nil
}

type serviceHookHandler struct{}

func (this serviceHookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var p patch
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	pvd.Update(p)
}

// #region 客户端依赖中心
type providers struct { // 客户端依赖中心
	services map[ServiceName][]string
	mutex    *sync.RWMutex
}

var pvd = providers{
	services: make(map[ServiceName][]string),
	mutex:    new(sync.RWMutex),
}

// 注册serviceHookURL，由注册中心客户端唤起该方法
func (this *providers) Update(pat patch) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	// 注册
	for i := 0; i < len(pat.Added); i++ {
		servName, servURL := pat.Added[i].Name, pat.Added[i].URL
		if _, ok := this.services[servName]; !ok {
			this.services[servName] = make([]string, 0)
		}
		this.services[servName] = append(this.services[servName], servURL)
	}
	// 移除
	for i := 0; i < len(pat.Removed); i++ {
		servName, servURL := pat.Removed[i].Name, pat.Removed[i].URL
		if urls, ok := this.services[servName]; ok {
			kept := urls[:0]
			for _, u := range urls {
				if u != servURL {
					kept = append(kept, u)
				}
			}
			if len(kept) == 0 {
				delete(this.services, servName)
			} else {
				this.services[servName] = kept
			}
		}
	}
}

// 根据服务名获得URL
func (p providers) get(name ServiceName) (string, error) {
	providers, ok := p.services[name]
	if !ok {
		return "", fmt.Errorf("No providers available for service %v", name)
	}
	idx := int(rand.Float32() * float32(len(providers)))
	return providers[idx], nil
}

// 提供给外部
func GetProvider(name ServiceName) (string, error) {
	return pvd.get(name)
}

//#endregion
