package email

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	stdurl "net/url"
	"strings"
	"time"

	xproxy "golang.org/x/net/proxy"
)

// dialThroughProxyContext 通过给定代理 URL 与目标 host:port 建立可取消的 TCP/SOCKS 连接。
// 支持的 scheme：http / https / socks5 / socks5h。proxyURL 为空时直连。
func dialThroughProxyContext(ctx context.Context, proxyURL, network, addr string, timeout time.Duration) (net.Conn, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if proxyURL == "" {
		return (&net.Dialer{Timeout: timeout}).DialContext(ctx, network, addr)
	}
	u, err := stdurl.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理地址解析失败: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "socks5", "socks5h":
		var auth *xproxy.Auth
		if u.User != nil {
			pwd, _ := u.User.Password()
			auth = &xproxy.Auth{User: u.User.Username(), Password: pwd}
		}
		d, err := xproxy.SOCKS5("tcp", u.Host, auth, &net.Dialer{Timeout: timeout})
		if err != nil {
			return nil, err
		}
		conn, err := d.(xproxy.ContextDialer).DialContext(ctx, network, addr)
		if ctx.Err() != nil {
			if conn != nil {
				conn.Close()
			}
			return nil, ctx.Err()
		}
		return conn, err
	case "http", "https":
		return dialHTTPConnect(ctx, u, addr)
	default:
		return nil, fmt.Errorf("不支持的代理协议: %s", u.Scheme)
	}
}

// dialHTTPConnect 通过 HTTP(S) 代理用 CONNECT 方法建立到目标的 TCP 隧道。
func dialHTTPConnect(ctx context.Context, u *stdurl.URL, target string) (net.Conn, error) {
	proxyAddr := u.Host
	if u.Port() == "" {
		port := "80"
		if strings.EqualFold(u.Scheme, "https") {
			port = "443"
		}
		proxyAddr = net.JoinHostPort(u.Hostname(), port)
	}
	var conn net.Conn
	var err error
	if strings.EqualFold(u.Scheme, "https") {
		dialer := tls.Dialer{Config: &tls.Config{ServerName: u.Hostname(), MinVersion: tls.VersionTLS12}}
		conn, err = dialer.DialContext(ctx, "tcp", proxyAddr)
	} else {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", proxyAddr)
	}
	if err != nil {
		return nil, err
	}
	stopCancel := context.AfterFunc(ctx, func() { conn.Close() })
	defer stopCancel()
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}

	req := "CONNECT " + target + " HTTP/1.1\r\nHost: " + target + "\r\n"
	if u.User != nil {
		pwd, _ := u.User.Password()
		token := base64.StdEncoding.EncodeToString([]byte(u.User.Username() + ":" + pwd))
		req += "Proxy-Authorization: Basic " + token + "\r\n"
	}
	req += "\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: "CONNECT"})
	if err != nil {
		conn.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("CONNECT 响应解析失败: %w", err)
	}
	if resp.StatusCode != 200 {
		conn.Close()
		return nil, fmt.Errorf("CONNECT 失败: %s", resp.Status)
	}
	if !stopCancel() || ctx.Err() != nil {
		conn.Close()
		return nil, ctx.Err()
	}
	conn.SetDeadline(time.Time{}) // 清掉握手 deadline
	if br.Buffered() > 0 {
		return &bufferedProxyConn{Conn: conn, reader: br}, nil
	}
	return conn, nil
}

type bufferedProxyConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedProxyConn) Read(p []byte) (int, error) { return c.reader.Read(p) }

// httpClientWithProxy 返回带代理的 http.Client（用于 OAuth refresh 等）。
func httpClientWithProxy(proxyURL string, timeout time.Duration) *http.Client {
	transport := &http.Transport{
		DialContext:     (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
		IdleConnTimeout: 90 * time.Second,
	}
	if proxyURL != "" {
		if u, err := stdurl.Parse(proxyURL); err == nil {
			switch strings.ToLower(u.Scheme) {
			case "http", "https":
				transport.Proxy = http.ProxyURL(u)
			case "socks5", "socks5h":
				transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dialThroughProxyContext(ctx, proxyURL, network, addr, 15*time.Second)
				}
			}
		}
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}
