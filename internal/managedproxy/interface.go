package managedproxy

import (
	"errors"
	"os"

	"go.yaml.in/yaml/v3"
)

// BindInterface changes only the core's host transport. MATCH routing and
// proxy credentials remain intact; this does not create a guest direct route.
func BindInterface(data []byte, name string) ([]byte, error) {
	if name == "" {
		return nil, errors.New("缺少物理网卡，不能启动代理")
	}
	var config map[string]any
	if err := yaml.Unmarshal(data, &config); err != nil || config == nil {
		return nil, errors.New("代理运行配置无效")
	}
	config["interface-name"] = name
	// Older imports may carry a node-level binding, which overrides the global.
	if nodes, ok := config["proxies"].([]any); ok {
		for _, item := range nodes {
			if node, ok := item.(map[string]any); ok {
				if _, bound := node["interface-name"]; bound {
					node["interface-name"] = name
				}
			}
		}
	}
	return yaml.Marshal(config)
}

func BoundInterface(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var config struct {
		Interface string `yaml:"interface-name"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", err
	}
	return config.Interface, nil
}
