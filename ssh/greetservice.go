package ssh

type GreetService struct {
	version string
}

// NewGreetService 鍒涘缓鏈嶅姟瀹炰緥
func NewGreetService(version string) *GreetService {
	return &GreetService{version: version}
}

func (g *GreetService) Greet(name string) string {
	return "Hello " + name + "!"
}

// GetVersion 鑾峰彇杞欢鐗堟湰
func (g *GreetService) GetVersion() string {
	return g.version
}

// GetAppName 鑾峰彇搴旂敤鍚嶇О
func (g *GreetService) GetAppName() string {
	return "鍚疭SH"
}

