package registry

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

// MultiServerDiscovery 一个不需要注册中心的、服务列表由手工维护的注册中心
// 实现了Discovery所有的接口
// r 是一个随机数，初始化时使用时间戳设定，避免每次都产生同一个随机数序列
// servers 注册中心中的服务列表
// index 记录轮询算法轮询到的位置，避免每次从相同的位置开始轮询
type MultiServerDiscovery struct {
	r       *rand.Rand
	mu      sync.RWMutex
	servers []string
	index   int
}

// NewMultiServerDiscovery 根据服务列表构造一个注册中心
func NewMultiServerDiscovery(servers []string) *MultiServerDiscovery {
	d := &MultiServerDiscovery{
		servers: servers,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	d.index = d.r.Intn(math.MaxInt32 - 1)
	return d
}

var _ Discovery = (*MultiServerDiscovery)(nil)

// Refresh 对于MultiServerDiscovery来说，由于是手动维护，Refresh没有作用
func (m *MultiServerDiscovery) Refresh() error {
	//TODO implement me
	return nil
}

func (m *MultiServerDiscovery) Update(servers []string) error {
	//TODO implement me
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers = servers
	return nil
}

func (m *MultiServerDiscovery) Get(mode BalanceMode) (string, error) {
	//TODO implement me
	m.mu.Lock()
	defer m.mu.Unlock()

	n := len(m.servers)
	if n == 0 {
		return "", errors.New("rpc discovery: no available servers")
	}
	switch mode {
	case RandomSelect:
		return m.servers[m.r.Intn(n)], nil
	case RoundRobinSelect:
		s := m.servers[m.index%n]
		m.index = (m.index + 1) % n
		return s, nil
	default:
		return "", errors.New("rpc discovery: not supported select mode")
	}
}

func (m *MultiServerDiscovery) GetAll() ([]string, error) {
	//TODO implement me
	m.mu.Lock()
	defer m.mu.Unlock()

	servers := make([]string, len(m.servers), len(m.servers))
	copy(servers, m.servers)
	return servers, nil
}
