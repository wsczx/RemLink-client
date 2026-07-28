package vpnc

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/jackpal/gateway"
	"sslcon/base"
	"sslcon/session"
	"sslcon/utils"
)

var VPNAddress string

func ConfigInterface(cSess *session.ConnSession) error {
	VPNAddress = cSess.VPNAddress
	cmdStr1 := fmt.Sprintf("ifconfig %s inet %s %s netmask %s up", cSess.TunName, cSess.VPNAddress, cSess.VPNAddress, "255.255.255.255")
	err := execCmd([]string{cmdStr1})
	if err != nil {
		return err
	}

	// IPv6 双栈：服务端分配了 v6 地址（/128 点对点）
	if cSess.VPNAddress6 != "" {
		cmdStr2 := fmt.Sprintf("ifconfig %s inet6 %s prefixlen 128", cSess.TunName, cSess.VPNAddress6)
		if e := execCmd([]string{cmdStr2}); e != nil {
			base.Error("add ipv6 address failed:", e)
		}
	}
	return nil
}

func SetRoutes(cSess *session.ConnSession) error {
	// 全局模式 = v4/v6 均未下发包含路由（v4 默认路由通过 DNS 的 OverridePrimary 设置）
	globalMode := len(cSess.SplitInclude) == 0 && len(cSess.SplitInclude6) == 0

	// 服务器直连路由：按地址族走对应的本地上行网关
	serverIP := net.ParseIP(cSess.ServerAddress)
	serverIsV6 := serverIP != nil && serverIP.To4() == nil
	if serverIsV6 {
		if gw6 := getGateway6(); gw6 != "" {
			cmdStr := fmt.Sprintf("route add -inet6 -host %s %s", cSess.ServerAddress, gw6)
			if e := execCmd([]string{cmdStr}); e != nil {
				return e
			}
		}
	} else {
		cmdStr1 := fmt.Sprintf("route add -host %s %s", cSess.ServerAddress, base.LocalInterface.Gateway)
		if e := execCmd([]string{cmdStr1}); e != nil {
			return e
		}
	}

	var err error
	// 默认路由通过 DNS 设置
	if len(cSess.SplitInclude) != 0 {
		for _, ipMask := range cSess.SplitInclude {
			dst := utils.IpMaskToCIDR(ipMask)
			cmdStr := fmt.Sprintf("route add -net %s %s", dst, cSess.VPNAddress)
			err = execCmd([]string{cmdStr})
			if err != nil {
				return routingError(dst, err)
			}
		}
	}

	if len(cSess.SplitExclude) > 0 {
		for _, ipMask := range cSess.SplitExclude {
			dst := utils.IpMaskToCIDR(ipMask)
			cmdStr := fmt.Sprintf("route add -net %s %s", dst, base.LocalInterface.Gateway)
			err = execCmd([]string{cmdStr})
			if err != nil {
				return routingError(dst, err)
			}
		}
	}

	// ===== IPv6 路由（服务端 -IP6 头，CIDR 格式）=====
	if cSess.VPNAddress6 != "" {
		include6 := cSess.SplitInclude6
		if len(include6) == 0 && globalMode {
			// 用两个 /1 覆盖全网而非 ::/0，避免与系统已有默认 v6 路由冲突
			include6 = []string{"::/1", "8000::/1"}
		}
		for _, cidr := range include6 {
			cmdStr := fmt.Sprintf("route add -inet6 -net %s -interface %s", cidr, cSess.TunName)
			if e := execCmd([]string{cmdStr}); e != nil {
				base.Error("add ipv6 include route failed:", cidr, e)
			}
		}
		if len(cSess.SplitExclude6) > 0 {
			if gw6 := getGateway6(); gw6 != "" {
				for _, cidr := range cSess.SplitExclude6 {
					cmdStr := fmt.Sprintf("route add -inet6 -net %s %s", cidr, gw6)
					_ = execCmd([]string{cmdStr})
				}
			} else {
				base.Info("no local ipv6 uplink, skip ipv6 exclude routes")
			}
		}
	}

	// dns
	if len(cSess.DNS) > 0 || len(cSess.DNS6) > 0 {
		err = setDNS(cSess)
	}

	return err
}

// getGateway6 解析本地默认 v6 网关（best-effort，无 v6 网络时返回空）。
// 注意须在给隧道添加 ::/0 之前调用才能取到物理上行的网关。
var cachedGateway6 string

func getGateway6() string {
	if cachedGateway6 != "" {
		return cachedGateway6
	}
	out, err := exec.Command("sh", "-c", "route -n get -inet6 default 2>/dev/null | awk '/gateway:/{print $2}'").Output()
	if err != nil {
		return ""
	}
	cachedGateway6 = strings.TrimSpace(string(out))
	return cachedGateway6
}

func ResetRoutes(cSess *session.ConnSession) {
	// cmdStr1 := fmt.Sprintf("route delete default %s", cSess.VPNAddress)
	// cmdStr2 := fmt.Sprintf("route add default %s", base.LocalInterface.Gateway)

	serverIP := net.ParseIP(cSess.ServerAddress)
	if serverIP != nil && serverIP.To4() == nil {
		cmdStr := fmt.Sprintf("route delete -inet6 -host %s", cSess.ServerAddress)
		_ = execCmd([]string{cmdStr})
	} else {
		cmdStr3 := fmt.Sprintf("route delete -host %s %s", cSess.ServerAddress, base.LocalInterface.Gateway)
		_ = execCmd([]string{cmdStr3})
	}

	// v6 路由清理（包含路由挂在 tun 接口上，随设备销毁自动消失；排除路由需显式删除）
	if len(cSess.SplitExclude6) > 0 {
		for _, cidr := range cSess.SplitExclude6 {
			cmdStr := fmt.Sprintf("route delete -inet6 -net %s", cidr)
			_ = execCmd([]string{cmdStr})
		}
	}

	if len(cSess.SplitExclude) > 0 {
		for _, ipMask := range cSess.SplitExclude {
			dst := utils.IpMaskToCIDR(ipMask)
			cmdStr := fmt.Sprintf("route delete -net %s %s", dst, base.LocalInterface.Gateway)
			_ = execCmd([]string{cmdStr})
		}
	}

	if len(cSess.DynamicSplitExcludeDomains) > 0 {
		cSess.DynamicSplitExcludeResolved.Range(func(_, value any) bool {
			ips := value.([]string)
			for _, ip := range ips {
				dst := ip + "/32"
				cmdStr := fmt.Sprintf("route delete -net %s %s", dst, base.LocalInterface.Gateway)
				_ = execCmd([]string{cmdStr})
			}

			return true
		})
	}

	if len(cSess.DNS) > 0 || len(cSess.DNS6) > 0 {
		restoreDNS(cSess)
	}
}

func DynamicAddIncludeRoutes(ips []string) {
	for _, ip := range ips {
		dst := ip + "/32"
		cmdStr := fmt.Sprintf("route add -net %s %s", dst, VPNAddress)
		_ = execCmd([]string{cmdStr})
	}
}

func DynamicAddExcludeRoutes(ips []string) {
	for _, ip := range ips {
		dst := ip + "/32"
		cmdStr := fmt.Sprintf("route add -net %s %s", dst, base.LocalInterface.Gateway)
		_ = execCmd([]string{cmdStr})
	}
}

func GetLocalInterface() error {
	localInterfaceIP, err := gateway.DiscoverInterface()
	if err != nil {
		return err
	}
	gateway, err := gateway.DiscoverGateway()
	if err != nil {
		return err
	}

	localInterface := net.Interface{}

	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipnet.IP.To4()
			if ip.Equal(localInterfaceIP) {
				localInterface = iface
				break
			}
		}
	}

	base.LocalInterface.Name = localInterface.Name
	base.LocalInterface.Ip4 = localInterfaceIP.String()
	base.LocalInterface.Gateway = gateway.String()
	base.LocalInterface.Mac = localInterface.HardwareAddr.String()

	base.Info("GetLocalInterface:", fmt.Sprintf("%+v", *base.LocalInterface))

	return nil
}

func routingError(dst string, err error) error {
	return fmt.Errorf("routing error: %s %s", dst, err)
}

func execCmd(cmdStrs []string) error {
	for _, cmdStr := range cmdStrs {
		cmd := exec.Command("sh", "-c", cmdStr)
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s %s %s", err, cmd.String(), string(stdoutStderr))
		}
	}
	return nil
}

func setDNS(cSess *session.ConnSession) error {

	if len(cSess.DynamicSplitIncludeDomains) > 0 {
		DynamicAddIncludeRoutes(cSess.DNS)
	}

	var override string
	// 如果包含路由为空必为全局路由，如果使用包含域名，则包含路由必须填写一个，如 dns 地址
	if len(cSess.SplitInclude) == 0 {
		override = "d.add OverridePrimary # 1"
	}

	// v4 + v6 DNS 合并下发（scutil ServerAddresses 原生支持 v6 地址）
	allDNS := append(append([]string{}, cSess.DNS...), cSess.DNS6...)

	command := fmt.Sprintf(`
		open
		d.init
		d.add ServerAddresses * %s
        d.add SearchOrder 1
        d.add SupplementalMatchDomains * ""
		set State:/Network/Service/%s/DNS

		d.init
		d.add Router %s
		d.add Addresses * %s
		d.add InterfaceName %s
        %s
		set State:/Network/Service/%s/IPv4
		close
	`, strings.Join(allDNS, " "), cSess.TunName, cSess.VPNAddress, cSess.VPNAddress, cSess.TunName, override, cSess.TunName)

	cmd := exec.Command("scutil")
	cmd.Stdin = strings.NewReader(command)

	// 执行命令并获取输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		base.Error(err, output)
	}
	return err
}

func restoreDNS(cSess *session.ConnSession) {
	command := fmt.Sprintf(`
        open
        remove State:/Network/Service/%s/IPv4
        remove State:/Network/Service/%s/DNS
        close
	`, cSess.TunName, cSess.TunName)

	cmd := exec.Command("scutil")
	cmd.Stdin = strings.NewReader(command)

	// 执行命令并获取输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		base.Error(err, output)
	}
}
