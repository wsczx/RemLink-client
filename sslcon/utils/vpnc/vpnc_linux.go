package vpnc

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"sslcon/base"
	"sslcon/session"
	"sslcon/utils"
)

var (
	localInterface netlink.Link
	iface          netlink.Link

	// 本地 v6 上行（用于 v6 排除路由和 v6 服务器直连路由）。
	// 必须在给 tun 添加 v6 默认路由之前探测，否则 RouteGet 会命中隧道自身。
	localGateway6      net.IP
	localGateway6Index int
	hasLocalGateway6   bool
)

func ConfigInterface(cSess *session.ConnSession) error {
	var err error
	iface, err = netlink.LinkByName(cSess.TunName)
	if err != nil {
		return err
	}
	// ip address
	_ = netlink.LinkSetUp(iface)
	_ = netlink.LinkSetMulticastOff(iface)

	addr, _ := netlink.ParseAddr(utils.IpMask2CIDR(cSess.VPNAddress, cSess.VPNMask))
	err = netlink.AddrAdd(iface, addr)
	if err != nil {
		return err
	}

	// IPv6 双栈：服务端分配了 v6 地址（/128 点对点）
	if cSess.VPNAddress6 != "" {
		// 新接口的 disable_ipv6 继承自 conf/default，部分系统默认禁用，须显式解除
		_ = os.WriteFile("/proc/sys/net/ipv6/conf/"+cSess.TunName+"/disable_ipv6", []byte("0"), 0644)
		if addr6, e := netlink.ParseAddr(cSess.VPNAddress6 + "/128"); e == nil {
			if e = netlink.AddrAdd(iface, addr6); e != nil {
				base.Error("add ipv6 address failed:", e)
			}
		}
	}
	return nil
}

func SetRoutes(cSess *session.ConnSession) error {
	ifaceIndex := iface.Attrs().Index
	localInterfaceIndex := localInterface.Attrs().Index
	gateway := net.ParseIP(base.LocalInterface.Gateway)

	// 探测本地 v6 上行（必须先于任何 v6 路由变更，否则 RouteGet 命中隧道）
	if cSess.VPNAddress6 != "" {
		probeGateway6()
	}

	// 服务器直连路由：按地址族走对应的本地上行网关
	serverIP := net.ParseIP(cSess.ServerAddress)
	if serverIP != nil && serverIP.To4() == nil {
		if hasLocalGateway6 {
			dst6, _ := netlink.ParseIPNet(cSess.ServerAddress + "/128")
			err := netlink.RouteAdd(&netlink.Route{LinkIndex: localGateway6Index, Dst: dst6, Gw: localGateway6})
			if err != nil && !strings.HasSuffix(err.Error(), "exists") {
				return routingError(dst6, err)
			}
		}
	} else {
		dst, _ := netlink.ParseIPNet(cSess.ServerAddress + "/32")
		route := netlink.Route{LinkIndex: localInterfaceIndex, Dst: dst, Gw: gateway}
		err := netlink.RouteAdd(&route)
		if err != nil {
			if !strings.HasSuffix(err.Error(), "exists") {
				return routingError(dst, err)
			}
		}
	}

	// 全局模式 = v4/v6 均未下发包含路由
	globalMode := len(cSess.SplitInclude) == 0 && len(cSess.SplitInclude6) == 0

	var dst *net.IPNet
	var route netlink.Route
	var err error

	// 如果包含路由为空必为全局路由，如果使用包含域名，则包含路由必须填写一个，如 dns 地址
	if len(cSess.SplitInclude) == 0 {
		cSess.SplitInclude = append(cSess.SplitInclude, "0.0.0.0/0.0.0.0")

		// 全局模式，重置默认路由优先级，如 OpenWrt 默认优先级为 0
		zero, _ := netlink.ParseIPNet("0.0.0.0/0")
		delAllRoute(&netlink.Route{LinkIndex: localInterfaceIndex, Dst: zero})
		_ = netlink.RouteAdd(&netlink.Route{LinkIndex: localInterfaceIndex, Dst: zero, Gw: gateway, Priority: 10})
	}

	// 如果使用域名包含，原则上不支持在顶级域名匹配中排除某个具体域名的 IP
	for _, ipMask := range cSess.SplitInclude {
		dst, _ = netlink.ParseIPNet(utils.IpMaskToCIDR(ipMask))
		route = netlink.Route{LinkIndex: ifaceIndex, Dst: dst, Priority: 6}
		err = netlink.RouteAdd(&route)
		if err != nil {
			if !strings.HasSuffix(err.Error(), "exists") {
				return routingError(dst, err)
			}
		}
	}

	// 支持在 SplitInclude 网段中排除某个路由
	if len(cSess.SplitExclude) > 0 {
		for _, ipMask := range cSess.SplitExclude {
			dst, _ = netlink.ParseIPNet(utils.IpMaskToCIDR(ipMask))
			route = netlink.Route{LinkIndex: localInterfaceIndex, Dst: dst, Gw: gateway, Priority: 5}
			err = netlink.RouteAdd(&route)
			if err != nil {
				if !strings.HasSuffix(err.Error(), "exists") {
					return routingError(dst, err)
				}
			}
		}
	}

	// ===== IPv6 路由（服务端 -IP6 头，CIDR 格式）=====
	if cSess.VPNAddress6 != "" {
		include6 := cSess.SplitInclude6
		if len(include6) == 0 && globalMode {
			include6 = []string{"::/0"}
		}
		for _, cidr := range include6 {
			dst6, e := netlink.ParseIPNet(cidr)
			if e != nil {
				continue
			}
			e = netlink.RouteAdd(&netlink.Route{LinkIndex: ifaceIndex, Dst: dst6, Priority: 6})
			if e != nil && !strings.HasSuffix(e.Error(), "exists") {
				base.Error("add ipv6 include route failed:", dst6.String(), e)
			}
		}
		// v6 排除路由回本地上行；无 v6 上行时跳过（不阻断连接）
		if len(cSess.SplitExclude6) > 0 {
			if hasLocalGateway6 {
				for _, cidr := range cSess.SplitExclude6 {
					dst6, e := netlink.ParseIPNet(cidr)
					if e != nil {
						continue
					}
					_ = netlink.RouteAdd(&netlink.Route{LinkIndex: localGateway6Index, Dst: dst6, Gw: localGateway6, Priority: 5})
				}
			} else {
				base.Info("no local ipv6 uplink, skip ipv6 exclude routes")
			}
		}
	}

	if len(cSess.DNS) > 0 || len(cSess.DNS6) > 0 {
		setDNS(cSess)
	}

	return nil
}

// probeGateway6 通过路由查询探测本地 v6 上行网关（best-effort，无 v6 网络时静默失败）
func probeGateway6() {
	hasLocalGateway6 = false
	routes, err := netlink.RouteGet(net.ParseIP("2400:3200::1"))
	if err != nil || len(routes) == 0 {
		return
	}
	r := routes[0]
	if r.Gw == nil {
		return
	}
	localGateway6 = r.Gw
	localGateway6Index = r.LinkIndex
	hasLocalGateway6 = true
}

func ResetRoutes(cSess *session.ConnSession) {
	// routes
	localInterfaceIndex := localInterface.Attrs().Index

	for _, ipMask := range cSess.SplitInclude {
		if ipMask == "0.0.0.0/0.0.0.0" {
			// 重置默认路由优先级
			zero, _ := netlink.ParseIPNet("0.0.0.0/0")
			gateway := net.ParseIP(base.LocalInterface.Gateway)
			_ = netlink.RouteDel(&netlink.Route{LinkIndex: localInterfaceIndex, Dst: zero})
			_ = netlink.RouteAdd(&netlink.Route{LinkIndex: localInterfaceIndex, Dst: zero, Gw: gateway})
			break
		}
	}

	// 服务器直连路由（按地址族）
	serverIP := net.ParseIP(cSess.ServerAddress)
	if serverIP != nil && serverIP.To4() == nil {
		if hasLocalGateway6 {
			dst6, _ := netlink.ParseIPNet(cSess.ServerAddress + "/128")
			_ = netlink.RouteDel(&netlink.Route{LinkIndex: localGateway6Index, Dst: dst6})
		}
	} else {
		dst4, _ := netlink.ParseIPNet(cSess.ServerAddress + "/32")
		_ = netlink.RouteDel(&netlink.Route{LinkIndex: localInterfaceIndex, Dst: dst4})
	}

	var dst *net.IPNet
	if len(cSess.SplitExclude) > 0 {
		for _, ipMask := range cSess.SplitExclude {
			dst, _ = netlink.ParseIPNet(utils.IpMaskToCIDR(ipMask))
			_ = netlink.RouteDel(&netlink.Route{LinkIndex: localInterfaceIndex, Dst: dst})
		}
	}

	// v6 排除路由（v6 包含路由挂在 tun 上，随设备销毁自动消失）
	if len(cSess.SplitExclude6) > 0 && hasLocalGateway6 {
		for _, cidr := range cSess.SplitExclude6 {
			dst6, e := netlink.ParseIPNet(cidr)
			if e != nil {
				continue
			}
			_ = netlink.RouteDel(&netlink.Route{LinkIndex: localGateway6Index, Dst: dst6})
		}
	}

	if len(cSess.DynamicSplitExcludeDomains) > 0 {
		cSess.DynamicSplitExcludeResolved.Range(func(_, value any) bool {
			ips := value.([]string)
			for _, ip := range ips {
				dst, _ = netlink.ParseIPNet(ip + "/32")
				_ = netlink.RouteDel(&netlink.Route{LinkIndex: localInterfaceIndex, Dst: dst})
			}

			return true
		})
	}

	if len(cSess.DNS) > 0 || len(cSess.DNS6) > 0 {
		restoreDNS(cSess)
	}
}

func DynamicAddIncludeRoutes(ips []string) {
	ifaceIndex := iface.Attrs().Index

	for _, ip := range ips {
		dst, _ := netlink.ParseIPNet(ip + "/32")
		route := netlink.Route{LinkIndex: ifaceIndex, Dst: dst, Priority: 6}
		_ = netlink.RouteAdd(&route)
	}
}

func DynamicAddExcludeRoutes(ips []string) {
	localInterfaceIndex := localInterface.Attrs().Index
	gateway := net.ParseIP(base.LocalInterface.Gateway)

	for _, ip := range ips {
		dst, _ := netlink.ParseIPNet(ip + "/32")
		route := netlink.Route{LinkIndex: localInterfaceIndex, Dst: dst, Gw: gateway, Priority: 5}
		_ = netlink.RouteAdd(&route)
	}
}

func GetLocalInterface() error {

	// just for default route
	routes, err := netlink.RouteGet(net.ParseIP("8.8.8.8"))
	if len(routes) > 0 {
		route := routes[0]
		localInterface, err = netlink.LinkByIndex(route.LinkIndex)
		if err != nil {
			return err
		}
		base.LocalInterface.Name = localInterface.Attrs().Name
		base.LocalInterface.Ip4 = route.Src.String()
		base.LocalInterface.Gateway = route.Gw.String()
		base.LocalInterface.Mac = localInterface.Attrs().HardwareAddr.String()

		base.Info("GetLocalInterface:", fmt.Sprintf("%+v", *base.LocalInterface))
		return nil
	}
	return err
}

func delAllRoute(route *netlink.Route) {
	err := netlink.RouteDel(route)
	if err != nil {
		return
	}
	delAllRoute(route)
}

func routingError(dst *net.IPNet, err error) error {
	return fmt.Errorf("routing error: %s %s", dst.String(), err)
}

func setDNS(cSess *session.ConnSession) {
	// dns（v4 + v6，resolv.conf 的 nameserver 原生支持 v6 地址）
	allDNS := append(append([]string{}, cSess.DNS...), cSess.DNS6...)
	if len(allDNS) > 0 {
		// 使用动态域名路由时 DNS 一定走 VPN 才能进行流量分析
		if len(cSess.DynamicSplitIncludeDomains) > 0 {
			DynamicAddIncludeRoutes(cSess.DNS)
		}

		// 部分云服务器会在设置路由时重写 /etc/resolv.conf，延迟两秒再设置
		go func() {
			utils.CopyFile("/tmp/resolv.conf.bak", "/etc/resolv.conf")

			var dnsString string
			for _, dns := range allDNS {
				dnsString += fmt.Sprintf("nameserver %s\n", dns)
			}
			time.Sleep(2 * time.Second)
			// OpenWrt 会将 127.0.0.1 写在最下面，影响其上面的解析
			err := utils.NewRecord("/etc/resolv.conf").Write(dnsString, false)
			if err != nil {
				base.Error("set DNS failed")
			}
		}()
	}
}

func restoreDNS(cSess *session.ConnSession) {
	// dns
	// 软件崩溃会导致无法恢复 resolv.conf 从而无法上网，需要重启系统
	if len(cSess.DNS) > 0 || len(cSess.DNS6) > 0 {
		utils.CopyFile("/etc/resolv.conf", "/tmp/resolv.conf.bak")
	}
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
