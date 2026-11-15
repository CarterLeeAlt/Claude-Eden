package localsession

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
)

type sessionTransport struct {
	base       *http.Transport
	jar        http.CookieJar
	origin     string
	ctx        context.Context
	invalidate func() error
}

func (t *sessionTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Scheme+"://"+request.URL.Host != t.origin || request.URL.User != nil {
		return nil, errors.New("拒绝访问本机控制服务之外的地址")
	}
	if err := t.ctx.Err(); err != nil {
		return nil, errors.New("本机工作台连接已断开")
	}
	ctx, cancel := context.WithCancel(request.Context())
	stop := context.AfterFunc(t.ctx, cancel)
	finish := func() { stop(); cancel() }
	req := request.Clone(ctx)
	req.Header = request.Header.Clone()
	req.Host = req.URL.Host
	req.Header.Del("Cookie")
	req.Header.Del("Authorization")
	req.Header.Del("Proxy-Authorization")
	req.Header.Del("Idempotency-Key")
	req.Header.Del("X-Idempotency-Key")
	req.GetBody = nil
	req.Header.Set("Origin", t.origin)
	for _, cookie := range t.jar.Cookies(req.URL) {
		req.AddCookie(cookie)
	}
	response, err := t.base.RoundTrip(req)
	if err != nil {
		finish()
		return nil, fmt.Errorf("本机请求中断，请检查连接；已提交的操作不会自动重发: %w", err)
	}
	if response.StatusCode == http.StatusUnauthorized {
		response.Body.Close()
		finish()
		if t.invalidate != nil {
			_ = t.invalidate()
		}
		return nil, errors.New("本机会话已失效，请重新连接并核对任务")
	}
	t.jar.SetCookies(req.URL, response.Cookies())
	response.Header.Del("Set-Cookie")
	if response.StatusCode >= 300 && response.StatusCode < 400 && response.Header.Get("Location") != "" {
		response.Body.Close()
		finish()
		return nil, errors.New("本机服务重定向已拒绝")
	}
	if rw, ok := response.Body.(io.ReadWriteCloser); ok {
		response.Body = &cancelReadWriteCloser{ReadWriteCloser: rw, finish: finish}
	} else {
		response.Body = &cancelReadCloser{ReadCloser: response.Body, finish: finish}
	}
	return response, nil
}

type cancelReadCloser struct {
	io.ReadCloser
	finish func()
	once   sync.Once
}

func (b *cancelReadCloser) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.once.Do(b.finish)
	}
	return n, err
}

func (b *cancelReadCloser) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.finish)
	return err
}

type cancelReadWriteCloser struct {
	io.ReadWriteCloser
	finish func()
	once   sync.Once
}

func (b *cancelReadWriteCloser) Read(p []byte) (int, error) {
	n, err := b.ReadWriteCloser.Read(p)
	if err != nil {
		b.once.Do(b.finish)
	}
	return n, err
}

func (b *cancelReadWriteCloser) Close() error {
	err := b.ReadWriteCloser.Close()
	b.once.Do(b.finish)
	return err
}
