/*
服务端和客户端对于注册中心的通信均采用的是HTTP协议。客户端GET服务列表，服务端POST服务实例和心跳
*/

package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type ServerItem struct {
	Addr  string
	start time.Time
}

/*
Registry 是一个简单的注册中心，提供以下功能：
1. 添加服务
2. 接收心跳
3. 返回可用服务
4. 删除不可用服务
*/
type Registry struct {
	timeout time.Duration // 超时时间默认为5min，超过就认为服务不可用
	mu      sync.Mutex
	servers map[string]*ServerItem
}

const (
	defaultPath    = "/rpc-test/registry"
	defaultTimeout = time.Minute * 5
)

func NewRegistry(timeout time.Duration) *Registry {
	return &Registry{
		timeout: timeout,
		servers: make(map[string]*ServerItem),
	}
}

var DefaultRegistry = NewRegistry(defaultTimeout)

// putServer 添加服务实例，如果服务已存在则更新start
func (r *Registry) putServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.servers[addr]
	if s != nil {
		s.start = time.Now()
	} else {
		r.servers[addr] = &ServerItem{addr, time.Now()}
	}
}

// aliveServers 返回可用的服务列表，如果存在超时的服务则删除
func (r *Registry) aliveServer() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var alive []string
	for addr, s := range r.servers {
		if r.timeout == 0 || s.start.Add(r.timeout).After(time.Now()) {
			alive = append(alive, addr)
		} else {
			delete(r.servers, addr)
		}
	}
	sort.Strings(alive)
	return alive
}

/*
ServerHTTP Registry采用 HTTP 协议提供服务，且所有的有用信息都承载在 HTTP Header 中
Get：返回所有可用的服务列表，通过自定义字段 X-rpc-Servers 承载。
Post：添加服务实例或发送心跳，通过自定义字段 X-rpc-Server 承载。
*/
func (r *Registry) ServerHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		w.Header().Set("X-rpc-Server", strings.Join(r.aliveServer(), ","))
	case "POST":
		addr := req.Header.Get("X-rpc-Server")
		// 提供的服务端地址url为空时
		if addr == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.putServer(addr)
	// 除了GET和POST外其他的HTTP方法无效
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
func (r *Registry) HandleHTTP(registryPath string) {
	http.Handle(registryPath, http.HandlerFunc(r.ServerHTTP))
	log.Println("rpc registry path: ", registryPath)
}

func HandleHTTP() {
	DefaultRegistry.HandleHTTP(defaultPath)
}

// Heartbeat 服务启动时定时向注册中心发送心跳，默认周期比注册中心设置的过期时间少 1 min。
func Heartbeat(registry, addr string, duration time.Duration) {
	if duration == 0 {
		// 确保在超时之前有足够的时间发送心跳
		duration = DefaultRegistry.timeout - time.Duration(1)*time.Minute
	}
	err := sendHeartbeat(registry, addr)
	go func() {
		t := time.NewTicker(duration) // 启动一个定时器
		// 只要不产生错误就一直定期发送心跳
		for err == nil {
			<-t.C
			err = sendHeartbeat(registry, addr)
		}
	}()
}

func sendHeartbeat(registry string, addr string) interface{} {
	log.Println(addr, " send heartbeat to registry ", registry)
	httpClient := &http.Client{}
	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set("X-rpc-Server", addr)
	if _, err := httpClient.Do(req); err != nil {
		log.Println("rpc server: heartbeat error:", err)
		return err
	}
	return nil
}
