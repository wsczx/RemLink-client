//go:build linux

package engine

import (
	"fmt"
	"os/exec"
	"strings"
)

// Linux 断网保护（P1d）：用 iptables 在 OUTPUT 链前置一个专用链，
// 仅放行回环 / 经隧道(tun+) 的流量 / 到 VPN 服务端端口的流量，其余明文出网 DROP。
//
// 真机调优点：
//   - 隧道接口名用 tun+ 通配（remlink/tun0 等），若服务端用其它前缀需调整；
//   - DNS 若走隧道(tun)已放行；若 Split-DNS 走本地解析器，需额外放行 53 以免误伤；
//   - 提供 DisableKillSwitch 紧急关闭兜底，避免把自己锁死。
const ksChain = "REMLINK_KS"

// EnableKillSwitch 启用断网保护。server 为 VPN 服务端 host，port 为其监听端口。
func EnableKillSwitch(server string, port int) error {
	// 建链（已存在则忽略错误）
	_ = runIPT("-N", ksChain)
	// 清空旧规则，避免重复叠加
	_ = runIPT("-F", ksChain)

	rules := [][]string{
		{"-A", ksChain, "-i", "lo", "-j", "ACCEPT"},
		{"-A", ksChain, "-o", "tun+", "-j", "ACCEPT"},
		{"-A", ksChain, "-p", "tcp", "-d", server, "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT"},
		{"-A", ksChain, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
		{"-A", ksChain, "-j", "DROP"},
	}
	for _, r := range rules {
		if err := runIPT(r...); err != nil {
			return fmt.Errorf("添加 Kill Switch 规则失败: %w", err)
		}
	}
	// OUTPUT 跳转到专用链（已存在跳转则忽略）
	if err := runIPT("-I", "OUTPUT", "-j", ksChain); err != nil {
		return fmt.Errorf("插入 OUTPUT 跳转失败: %w", err)
	}
	return nil
}

// DisableKillSwitch 关闭断网保护：删 OUTPUT 跳转并清空专用链。
func DisableKillSwitch() error {
	_ = runIPT("-D", "OUTPUT", "-j", ksChain)
	_ = runIPT("-F", ksChain)
	_ = runIPT("-X", ksChain)
	return nil
}

func runIPT(args ...string) error {
	cmd := exec.Command("iptables", args...)
	out, err := cmd.CombinedOutput()
	if err != nil && strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return err
}
