/*
负载均衡有很多策略，这里只提供和两种策略：随机选择Random和轮询策略RoundRobin
discovery.go是一个简单的服务发现模块
*/

package xclient

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

// SelectMode 不同的负载均衡策略
type SelectMode int

// SelectMode 定义一个枚举类型，包含两种负载均衡策略
const (
	RandomSelect SelectMode = iota
	RoundRobinSelect
)

/*
Discovery 是一个接口类型，包含了服务发现所需要的最基本的接口。
Refresh() 从注册中心更新服务列表
Update(servers []string) 手动更新服务列表
Get(mode SelectMode) 根据负载均衡策略，选择一个服务实例
GetAll() 返回所有的服务实例
*/
type Discovery interface {
	Refresh() error
	Update(servers []string) error
	Get(mode SelectMode) (string, error)
	GetAll() ([]string, error)
}

/*
MultiServerDiscovery 一个不需要注册中心的、服务列表由手工维护的注册中心
r 是一个随机数，初始化时使用时间戳设定，避免每次都产生同一个随机数序列
index 记录轮询算法轮询到的位置，避免每次从相同的位置开始轮询
*/
type MultiServerDiscovery struct {
	r       *rand.Rand
	mu      sync.RWMutex
	servers []string
	index   int
}

var _ Discovery = (*MultiServerDiscovery)(nil)

// NewMultiServerDiscovery 根据服务列表构造一个注册中心
func NewMultiServerDiscovery(servers []string) *MultiServerDiscovery {
	d := &MultiServerDiscovery{
		servers: servers,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	d.index = d.r.Intn(math.MaxInt32 - 1)
	return d
}

// Refresh 对于MultiServerDiscovery来说，由于是手动维护，Refresh没有作用
func (m *MultiServerDiscovery) Refresh() error {
	return nil
}

func (m *MultiServerDiscovery) Update(servers []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers = servers
	return nil
}

func (m *MultiServerDiscovery) Get(mode SelectMode) (string, error) {
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
	m.mu.Lock()
	defer m.mu.Unlock()

	servers := make([]string, len(m.servers), len(m.servers))
	copy(servers, m.servers)
	return servers, nil
}
