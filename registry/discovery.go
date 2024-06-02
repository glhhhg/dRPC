package registry

// BalanceMode 不同的负载均衡策略
type BalanceMode int

// BalanceMode 定义一个枚举类型，包含两种负载均衡策略
const (
	RandomSelect BalanceMode = iota
	RoundRobinSelect
)

// Discovery 是一个接口类型，包含了服务发现所需要的最基本的接口。
// Refresh() 从注册中心更新服务列表
// Update(servers []string) 手动更新服务列表
// Get(mode BalanceMode) 根据负载均衡策略，选择一个服务实例
// GetAll() 返回所有的服务实例
type Discovery interface {
	Refresh() error
	Update(servers []string) error
	Get(mode BalanceMode) (string, error)
	GetAll() ([]string, error)
}
