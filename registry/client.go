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
	"time"
)

func RegisterService(r Registration) error {
	serviceHookURL, err := url.Parse(r.ServiceHookURL)
	if err != nil {
		return err
	}
	http.Handle(serviceHookURL.Path, &serviceHookHandler{}) // 注册钩子处理函数（接收注册中心Patch）

	// 向注册中心发送自己的URL
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
	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
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
	fmt.Printf("Updated received %v\n", p)
	pvd.Update(p) // 更新本地Provider依赖
}

// #region 客户端依赖中心
type providers struct { // 客户端依赖中心
	services          map[ServiceName][]string
	watchers          map[ServiceName][]func()
	mutex             *sync.RWMutex
	notificationMutex *sync.Mutex
}

var pvd = providers{
	services:          make(map[ServiceName][]string),
	watchers:          make(map[ServiceName][]func()),
	mutex:             new(sync.RWMutex), // 保护共享数据
	notificationMutex: new(sync.Mutex),   // 保护通知和回调执行顺序
}

// 注册serviceHookURL，由注册中心客户端唤起该方法
func (this *providers) Update(pat patch) {
	changed := this.applyPatch(pat)
	for name := range changed {
		this.notify(name)
	}
}

// applyPatch 在锁内应用 Patch，返回发生变化的服务名。
// 锁在函数返回时释放，保证通知回调不会在持有 p.mutex 时执行。
func (this *providers) applyPatch(pat patch) map[ServiceName]struct{} {
	this.mutex.Lock()
	defer this.mutex.Unlock()

	changed := make(map[ServiceName]struct{}) // Set集合存储需要通知的服务名
	// 注册
	for i := 0; i < len(pat.Added); i++ {
		servName, servURL := pat.Added[i].Name, pat.Added[i].URL
		if _, ok := this.services[servName]; !ok {
			this.services[servName] = make([]string, 0)
		}
		this.services[servName] = append(this.services[servName], servURL)
		changed[servName] = struct{}{}
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
		changed[servName] = struct{}{}
	}
	return changed
}

// 根据服务名获得URL
func (p providers) get(name ServiceName) (string, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	providers, ok := p.services[name]
	if !ok || len(providers) == 0 {
		return "", fmt.Errorf("No providers available for service %v", name)
	}
	idx := int(rand.Float32() * float32(len(providers)))
	return providers[idx], nil
}

// 提供给外部
func GetProvider(name ServiceName) (string, error) {
	return pvd.get(name)
}

// OnProviderChange calls callback immediately and after each update to name.
func OnProviderChange(name ServiceName, callback func()) {
	pvd.mutex.Lock()
	pvd.watchers[name] = append(pvd.watchers[name], callback)
	pvd.mutex.Unlock()
	pvd.notify(name)
}

// notify 串行执行 name 的所有回调。notificationMutex 保证回调不并发执行，
// 从而避免回调并发操作全局状态（如 stdlog）产生竞态。
// 注意：回调在持有 notificationMutex 时执行，因此回调内不得再次触发通知
// （例如调用 OnProviderChange 或 Update），否则会死锁。
func (p *providers) notify(name ServiceName) {
	p.notificationMutex.Lock()
	defer p.notificationMutex.Unlock()

	p.mutex.RLock()
	callbacks := append([]func(){}, p.watchers[name]...) // 先复制，避免直接遍历执行callback时候长时间占用锁
	p.mutex.RUnlock()
	for _, callback := range callbacks {
		callback()
	}
}

//#endregion
