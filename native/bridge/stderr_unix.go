//go:build cgo && !windows

package main

import (
	"errors"
	"os"
)

var errStderrUnsupported = errors.New("当前平台不做标准错误重定向")

// attachStderr 在非 Windows 平台直接声明不支持。
//
// 原因：Windows 桌面端是唯一会把 Go fatal / panic 堆栈彻底丢掉的场景（双击启动
// 没有 console，stderr 直接消失）。Android 由 logcat 兜底，iOS 的 c-archive 由
// 宿主进程接管；而且 linux/arm64 上 syscall.Dup2 根本不存在（arm64 只有 dup3），
// 为免引入架构相关的编译风险，这里不做重定向——行为不变，不会造成回归。
func attachStderr(_ *os.File) error {
	return errStderrUnsupported
}
