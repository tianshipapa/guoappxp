package core

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"
)

// 投屏（DLNA）只需要一个电视能访问到的播放地址。核心本来就把每个在线播放
// 会话挂在本机 127.0.0.1 的流服务上（加请求头、解密分片、重写播放列表），
// 但电视拿不到手机的 127.0.0.1，所以投屏时再把同一个 handler 挂一份到
// 0.0.0.0，返回「内网 IP + 该端口」给 Flutter 拼出真正的投屏地址。

// nativeCastBase 打开（或复用）投屏监听，返回局域网可访问的基址。
func (stream *nativeStreamServer) nativeCastBase() (string, error) {
	stream.mu.Lock()
	if stream.castBase != "" {
		base := stream.castBase
		stream.mu.Unlock()
		return base, nil
	}
	address := nativeLocalIPv4()
	if address == "" {
		stream.mu.Unlock()
		return "", errors.New("当前没有可用的局域网地址，请先连接 Wi-Fi")
	}
	listener, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		stream.mu.Unlock()
		return "", errors.New("无法开启投屏端口")
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if port <= 0 {
		_ = listener.Close()
		stream.mu.Unlock()
		return "", errors.New("无法开启投屏端口")
	}
	server := &http.Server{
		Handler:           http.HandlerFunc(stream.nativeServe),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16384,
	}
	go func() { _ = server.Serve(listener) }()
	base := "http://" + net.JoinHostPort(address, strconv.Itoa(port))
	stream.castServer = server
	stream.castListener = listener
	stream.castBase = base
	stream.mu.Unlock()
	stream.nativeLog(nativeStreamDiagnostic{}, "cast_open", base, "", 0, "")
	return base, nil
}

// nativeCloseCast 停止投屏监听。会话本身不动，本机播放照常。
func (stream *nativeStreamServer) nativeCloseCast() {
	stream.mu.Lock()
	server := stream.castServer
	base := stream.castBase
	stream.castServer = nil
	stream.castListener = nil
	stream.castBase = ""
	stream.mu.Unlock()
	if server != nil {
		stream.nativeLog(nativeStreamDiagnostic{}, "cast_close", base, "", 0, "")
		_ = server.Close()
	}
}

// nativeLocalIPv4 挑一个电视能访问的内网地址：优先私有网段，避开回环、
// 链路本地与虚拟网卡常见的 169.254。
func nativeLocalIPv4() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	fallback := ""
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addresses {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			if ip.IsPrivate() {
				return ip.String()
			}
			if fallback == "" {
				fallback = ip.String()
			}
		}
	}
	return fallback
}

func (engine *nativeEngine) nativeCastBase() (string, error) {
	engine.mu.Lock()
	stream := engine.stream
	engine.mu.Unlock()
	if stream == nil {
		return "", errors.New("播放器尚未初始化，请先开始播放")
	}
	return stream.nativeCastBase()
}

func (engine *nativeEngine) nativeCloseCast() {
	engine.mu.Lock()
	stream := engine.stream
	engine.mu.Unlock()
	if stream != nil {
		stream.nativeCloseCast()
	}
}
