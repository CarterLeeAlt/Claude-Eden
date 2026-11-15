package workbench

import (
	"errors"
	"net/http"
)

type connectionTransport struct {
	base http.RoundTripper
	id   string
}

func (t connectionTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	copy.Header.Set(connectionHeader, t.id)
	return t.base.RoundTrip(copy)
}

func (t connectionTransport) CloseIdleConnections() {
	if closer, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

// A native picker captures the connection before the user selects a file.
// Matching it here prevents a slow picker from sending that file to a different
// controller generation after the user reconnects the workbench.
func (c *controllerClient) bindConnection(expected string) (ClientContext, error) {
	var current ClientContext
	status, err := c.call("GET", "/api/client/context", nil, &current)
	if err != nil || status != http.StatusOK {
		return current, errors.New("无法核对当前文件传递连接")
	}
	if current.DesktopGateway && (expected == "" || current.Mode != "local" || current.State.State != "connected") {
		return current, errors.New("请先选择工作环境，再重新进行文件操作")
	}
	if expected != "" && expected != current.ConnectionID {
		return current, errors.New("连接已经切换，请在当前主机重新选择文件")
	}
	c.connectionID = current.ConnectionID
	c.client.Transport = connectionTransport{base: c.client.Transport, id: current.ConnectionID}
	return current, nil
}
