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
	timeout time.Duration
	mu      sync.Mutex
	servers map[string]*ServerItem
}

const (
	defaultPath    = "/dRPC/registry"
	defaultTimeout = time.Minute * 5 // 超时时间默认为5min，超过就认为服务不可用
)

// NewRegistry 设置有效期限，创建一个Registry实例
func NewRegistry(timeout time.Duration) *Registry {
	return &Registry{
		timeout: timeout,
		servers: make(map[string]*ServerItem),
	}
}

// DefaultRegistry 根据默认时间创建默认的注册中心
var DefaultRegistry = NewRegistry(defaultTimeout)

// setServer 添加服务实例，如果服务已存在则更新start
func (r *Registry) setServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.servers[addr]
	if s != nil {
		s.start = time.Now()
	} else {
		r.servers[addr] = &ServerItem{addr, time.Now()}
	}
}

// getAliveServer 返回可用的服务列表，如果存在超时的服务则删除
func (r *Registry) getAliveServer() []string {
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

func (r *Registry) serverHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		w.Header().Set("Registry", strings.Join(r.getAliveServer(), ","))
	case "POST":
		addr := req.Header.Get("Registry")
		// 提供的服务端地址url为空时
		if addr == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.setServer(addr)
	// 除了GET和POST外其他的HTTP方法无效
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Registry) HandleHTTP(registryPath ...string) {
	var path string
	if len(registryPath) == 0 || registryPath[0] == "" {
		path = defaultPath
	} else if len(registryPath) == 1 {
		path = registryPath[0]
	} else {
		log.Println("rpc registry error: invalid registry path:", registryPath)
		return
	}
	http.Handle(path, http.HandlerFunc(r.serverHTTP))
	log.Println("rpc registry path: ", path)
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
	log.Println(addr, " send heartbeat to registry ", registry)
	httpClient := &http.Client{}
	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set("Registry", addr)
	if _, err := httpClient.Do(req); err != nil {
		log.Println("rpc server: heartbeat error:", err)
		return err
	}
	return nil
}
