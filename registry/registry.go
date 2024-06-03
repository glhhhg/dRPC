package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// ServerItem 存储注册中心中包含的所有服务器的地址和有效时间
type ServerItem struct {
	Addr  string
	start time.Time
}

// Registry 是一个简单的注册中心，提供以下功能：
// 添加服务, 接收心跳,返回可用服务,删除不可用服务
type Registry struct {
	timeout time.Duration // 超时时间默认为5min，超过就认为服务不可用
	mu      sync.Mutex
	servers map[string]*ServerItem
}

const (
	defaultPath    = "/rpc-test/registry"
	defaultTimeout = time.Minute * 5
)

// NewRegistry 设置有效期限，创建一个Registry实例
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
	log.Printf("rpc registry: aliveServer: %v", alive)
	return alive
}

// ServerHTTP Registry采用 HTTP 协议提供服务，且所有的有用信息都承载在 HTTP Header 中
// GET：返回所有可用的服务列表，通过自定义字段 X-rpc-Servers 承载。
// POST：添加服务实例或发送心跳，通过自定义字段 X-rpc-Server 承载。
func (r *Registry) ServerHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		w.Header().Set("X-rpc-Server", strings.Join(r.aliveServer(), ","))
	case "POST":
		addr := req.Header.Get("X-rpc-Server")
		log.Println("rpc registry: receive heartbeat from ", addr)
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
	log.Println("rpc registry: rpc registry handler path: ", registryPath)
}

func HandleHTTP() {
	DefaultRegistry.HandleHTTP(defaultPath)
}

// Heartbeat 提供给服务端的封装，服务启动时定时向注册中心发送心跳
// 默认发送心跳的周期比注册中心设置的过期时间少1min，确保数据能够送达。
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
	log.Printf("server %s send heartbeat to registry %s\n", addr, registry)
	httpClient := &http.Client{}
	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set("X-rpc-Server", addr)
	if _, err := httpClient.Do(req); err != nil {
		log.Println("rpc server: heartbeat error:", err)
		return err
	}
	return nil
}
