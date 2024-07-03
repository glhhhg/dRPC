// 定义请求和回复的结构体

package methods

type Args struct {
	A, B int
}

type Reply int

// Function 结构体
type Function struct{}

// Add 方法
func (s *Function) Add(args *Args, reply *Reply) error {
	*reply = Reply(args.A + args.B)
	return nil
}

// Subtract 方法
func (s *Function) Subtract(args *Args, reply *Reply) error {
	*reply = Reply(args.A - args.B)
	return nil
}

// Multiply 方法
func (s *Function) Multiply(args *Args, reply *Reply) error {
	*reply = Reply(args.A * args.B)
	return nil
}

// Divide 方法
func (s *Function) Divide(args *Args, reply *Reply) error {
	*reply = Reply(args.A / args.B)
	return nil
}

// Mod 方法
func (s *Function) Mod(args *Args, reply *Reply) error {
	*reply = Reply(args.A % args.B)
	return nil
}

// Min 方法
func (s *Function) Min(args *Args, reply *Reply) error {
	*reply = Reply(min(args.A, args.B))
	return nil
}

// Max 方法
func (s *Function) Max(args *Args, reply *Reply) error {
	*reply = Reply(max(args.A, args.B))
	return nil
}

// Power 方法
func (s *Function) Power(args *Args, reply *Reply) error {
	*reply = Reply(args.A ^ args.B)
	return nil
}

// GCD 方法
func (s *Function) GCD(args *Args, reply *Reply) error {
	*reply = Reply(gcd(args.A, args.B))
	return nil
}

// LCM 方法
func (s *Function) LCM(args *Args, reply *Reply) error {
	*reply = Reply(lcm(args.A, args.B))
	return nil
}

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func lcm(a, b int) int {
	return a * b / gcd(a, b)
}
