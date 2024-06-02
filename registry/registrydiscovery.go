package registry

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// RegistryDiscovery 继承了MultiServersDiscovery
// registry 即注册中心的地址
// timeout 服务列表的过期时间
// lastUpdate 是代表最后从注册中心更新服务列表的时间，默认10s过期，即10s之后，需要从注册中心更新新的列表。
type RegistryDiscovery struct {
	*MultiServerDiscovery
	registry   string
	timeout    time.Duration
	lastUpdate time.Time
}

const defaultUpdateTimeout = time.Second * 10

func NewRegistryDiscovery(registryAddr string, timeout time.Duration) *RegistryDiscovery {
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}
	d := &RegistryDiscovery{
		MultiServerDiscovery: NewMultiServerDiscovery(make([]string, 0)),
		registry:             registryAddr,
		timeout:              timeout,
	}
	return d
}
func (r *RegistryDiscovery) Update(servers []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.servers = servers
	r.lastUpdate = time.Now()
	return nil
}

func (r *RegistryDiscovery) Refresh() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lastUpdate.Add(r.timeout).After(time.Now()) {
		return nil
	}

	log.Println("rpc registry: refresh servers from registry: ", r.registry)
	resp, err := http.Get(r.registry)
	if err != nil {
		log.Println("rpc registry refresh error: ", err)
		return err
	}
	servers := strings.Split(resp.Header.Get("Registry"), ",")
	r.servers = make([]string, 0, len(servers))
	for _, server := range servers {
		if strings.TrimSpace(server) != "" {
			r.servers = append(r.servers, strings.TrimSpace(server))
		}
	}
	r.lastUpdate = time.Now()
	return nil
}

func (r *RegistryDiscovery) Get(mode BalanceMode) (string, error) {
	if err := r.Refresh(); err != nil {
		return "", err
	}
	return r.MultiServerDiscovery.Get(mode)
}

func (r *RegistryDiscovery) GetAll() ([]string, error) {
	if err := r.Refresh(); err != nil {
		return nil, err
	}
	return r.MultiServerDiscovery.GetAll()
}
