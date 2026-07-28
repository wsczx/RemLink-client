package vpnc

import (
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
	"sslcon/base"
	"sslcon/session"
	"sslcon/tun"
	"sslcon/utils"
)

var (
	localInterface  winipcfg.LUID
	iface           winipcfg.LUID
	nextHopVPN      netip.Addr
	nextHopGateway  netip.Addr
	nextHopGateway6 netip.Addr // 本地 v6 上行网关（用于 v6 排除路由/服务器 v6 直连）
	hasGateway6     bool
)

func ConfigInterface(cSess *session.ConnSession) error {
	mtu, _ := tun.NativeTunDevice.MTU()
	err := SetMTU(cSess.TunName, mtu)
	if err != nil {
		return err
	}

	iface = winipcfg.LUID(tun.NativeTunDevice.LUID())

	// ip address
	iface.FlushIPAddresses(windows.AF_UNSPEC)

	nextHopVPN, _ = netip.ParseAddr(cSess.VPNAddress)
	prefixVPN, _ := netip.ParsePrefix(utils.IpMask2CIDR(cSess.VPNAddress, cSess.VPNMask))
	err = iface.SetIPAddressesForFamily(windows.AF_INET, []netip.Prefix{prefixVPN})
	if err != nil {
		return err
	}

	// IPv6 双栈：服务端分配了 v6 地址（/128 点对点）
	if cSess.VPNAddress6 != "" {
		if prefixVPN6, e := netip.ParsePrefix(cSess.VPNAddress6 + "/128"); e == nil {
			if e = iface.SetIPAddressesForFamily(windows.AF_INET6, []netip.Prefix{prefixVPN6}); e != nil {
				base.Error("add ipv6 address failed:", e)
			}
		}
	}
	return nil
}

func SetRoutes(cSess *session.ConnSession) error {
	nextHopGateway, _ = netip.ParseAddr(base.LocalInterface.Gateway)

	// 全局模式 = v4/v6 均未下发包含路由
	globalMode := len(cSess.SplitInclude) == 0 && len(cSess.SplitInclude6) == 0

	// 服务器直连路由：按地址族走对应的本地上行网关
	var dst netip.Prefix
	var err error
	if serverIP, e := netip.ParseAddr(cSess.ServerAddress); e == nil && serverIP.Is6() {
		if hasGateway6 {
			dst, _ = netip.ParsePrefix(cSess.ServerAddress + "/128")
			err = localInterface.AddRoute(dst, nextHopGateway6, 5)
			if err != nil && !strings.HasSuffix(err.Error(), "exists.") {
				return routingError(dst, err)
			}
		}
	} else {
		dst, _ = netip.ParsePrefix(cSess.ServerAddress + "/32")
		err = localInterface.AddRoute(dst, nextHopGateway, 5)
		if err != nil {
			if !strings.HasSuffix(err.Error(), "exists.") {
				return routingError(dst, err)
			}
		}
	}

	// Windows 排除路由 metric 相对大小好像不起作用，但不影响效果
	if len(cSess.SplitInclude) == 0 {
		cSess.SplitInclude = append(cSess.SplitInclude, "0.0.0.0/0.0.0.0")
	}
	for _, ipMask := range cSess.SplitInclude {
		dst, _ = netip.ParsePrefix(utils.IpMaskToCIDR(ipMask))
		err = iface.AddRoute(dst, nextHopVPN, 6)
		if err != nil {
			if !strings.HasSuffix(err.Error(), "exists.") {
				return routingError(dst, err)
			}
		}
	}

	if len(cSess.SplitExclude) > 0 {
		for _, ipMask := range cSess.SplitExclude {
			dst, _ = netip.ParsePrefix(utils.IpMaskToCIDR(ipMask))
			err = localInterface.AddRoute(dst, nextHopGateway, 5)
			if err != nil {
				if !strings.HasSuffix(err.Error(), "exists.") {
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
			dst6, e := netip.ParsePrefix(cidr)
			if e != nil {
				continue
			}
			// 隧道 v6 路由为 on-link，下一跳用未指定地址
			e = iface.AddRoute(dst6, netip.IPv6Unspecified(), 6)
			if e != nil && !strings.HasSuffix(e.Error(), "exists.") {
				base.Error("add ipv6 include route failed:", dst6.String(), e)
			}
		}
		if len(cSess.SplitExclude6) > 0 {
			if hasGateway6 {
				for _, cidr := range cSess.SplitExclude6 {
					dst6, e := netip.ParsePrefix(cidr)
					if e != nil {
						continue
					}
					_ = localInterface.AddRoute(dst6, nextHopGateway6, 5)
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

func ResetRoutes(cSess *session.ConnSession) {
	// 服务器直连路由（按地址族）；隧道 v6 包含路由挂在 tun 上，随设备销毁自动消失
	var dst netip.Prefix
	if serverIP, e := netip.ParseAddr(cSess.ServerAddress); e == nil && serverIP.Is6() {
		if hasGateway6 {
			dst, _ = netip.ParsePrefix(cSess.ServerAddress + "/128")
			localInterface.DeleteRoute(dst, nextHopGateway6)
		}
	} else {
		dst, _ = netip.ParsePrefix(cSess.ServerAddress + "/32")
		localInterface.DeleteRoute(dst, nextHopGateway)
	}

	if len(cSess.SplitExclude) > 0 {
		for _, ipMask := range cSess.SplitExclude {
			dst, _ = netip.ParsePrefix(utils.IpMaskToCIDR(ipMask))
			localInterface.DeleteRoute(dst, nextHopGateway)
		}
	}

	if len(cSess.SplitExclude6) > 0 && hasGateway6 {
		for _, cidr := range cSess.SplitExclude6 {
			dst6, e := netip.ParsePrefix(cidr)
			if e != nil {
				continue
			}
			localInterface.DeleteRoute(dst6, nextHopGateway6)
		}
	}

	if len(cSess.DynamicSplitExcludeDomains) > 0 {
		cSess.DynamicSplitExcludeResolved.Range(func(_, value any) bool {
			ips := value.([]string)
			for _, ip := range ips {
				dst, _ = netip.ParsePrefix(ip + "/32")
				localInterface.DeleteRoute(dst, nextHopGateway)
			}

			return true
		})
	}
}

func DynamicAddIncludeRoutes(ips []string) {
	for _, ip := range ips {
		dst, _ := netip.ParsePrefix(ip + "/32")
		_ = iface.AddRoute(dst, nextHopVPN, 6)
	}
}

func DynamicAddExcludeRoutes(ips []string) {
	for _, ip := range ips {
		dst, _ := netip.ParsePrefix(ip + "/32")
		_ = localInterface.AddRoute(dst, nextHopGateway, 5)
	}
}

func GetLocalInterface() error {
	ifcs, err := winipcfg.GetAdaptersAddresses(windows.AF_INET, winipcfg.GAAFlagIncludeGateways)
	if err != nil {
		return err
	}

	var primaryInterface *winipcfg.IPAdapterAddresses
	var virtualPrimaryInterface *winipcfg.IPAdapterAddresses
	for _, ifc := range ifcs {
		base.Debug(ifc.AdapterName(), ifc.Description(), ifc.FriendlyName(), ifc.Ipv4Metric, ifc.IfType)
		// exclude Virtual Ethernet and Loopback Adapter
		// https://git.zx2c4.com/wireguard-windows/tree/tunnel/winipcfg/types.go?h=v0.5.3#n61
		if (ifc.IfType == 6 || ifc.IfType == 71) && ifc.FirstGatewayAddress != nil {
			if primaryInterface == nil || (ifc.Ipv4Metric < primaryInterface.Ipv4Metric) {
				if !strings.Contains(ifc.Description(), "Virtual") {
					primaryInterface = ifc
				} else if virtualPrimaryInterface == nil {
					virtualPrimaryInterface = ifc
				}
			}
		}
	}

	if primaryInterface == nil {
		if virtualPrimaryInterface != nil {
			primaryInterface = virtualPrimaryInterface
		} else {
			return fmt.Errorf("unable to find a valid network interface")
		}
	}

	base.Info("GetLocalInterface:", primaryInterface.AdapterName(), primaryInterface.Description(),
		primaryInterface.FriendlyName(), primaryInterface.Ipv4Metric, primaryInterface.IfType)

	base.LocalInterface.Name = primaryInterface.FriendlyName()
	base.LocalInterface.Ip4 = primaryInterface.FirstUnicastAddress.Address.IP().String()
	base.LocalInterface.Gateway = primaryInterface.FirstGatewayAddress.Address.IP().String()
	base.LocalInterface.Mac = net.HardwareAddr(primaryInterface.PhysicalAddress()).String()

	localInterface = primaryInterface.LUID

	// 探测同一适配器的 v6 上行网关（best-effort，无 v6 网络时静默失败）
	hasGateway6 = false
	if ifcs6, e := winipcfg.GetAdaptersAddresses(windows.AF_INET6, winipcfg.GAAFlagIncludeGateways); e == nil {
		for _, ifc := range ifcs6 {
			if ifc.LUID == localInterface && ifc.FirstGatewayAddress != nil {
				if gw6, ok := netip.AddrFromSlice(ifc.FirstGatewayAddress.Address.IP()); ok && gw6.Is6() {
					nextHopGateway6 = gw6
					hasGateway6 = true
				}
				break
			}
		}
	}

	return nil
}

func SetMTU(ifname string, mtu int) error {
	cmdStr := fmt.Sprintf("netsh interface ipv4 set subinterface \"%s\" MTU=%d", ifname, mtu)
	err := execCmd([]string{cmdStr})
	// v6 子接口 MTU：系统禁用 v6 时会失败，忽略错误
	_ = execCmd([]string{fmt.Sprintf("netsh interface ipv6 set subinterface \"%s\" MTU=%d", ifname, mtu)})
	return err
}

func routingError(dst netip.Prefix, err error) error {
	return fmt.Errorf("routing error: %s %s", dst.String(), err)
}

func execCmd(cmdStrs []string) error {
	for _, cmdStr := range cmdStrs {
		cmd := exec.Command("cmd", "/C", cmdStr)
		// reset env
		cmd.Env = []string{"Path=C:\\WINDOWS\\system32;C:\\WINDOWS"}
		// println(strings.Join(cmd.Env, "\n"))
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

	var servers []netip.Addr
	for _, dns := range cSess.DNS {
		addr, _ := netip.ParseAddr(dns)
		servers = append(servers, addr)
	}

	var err error
	if len(servers) > 0 {
		err = iface.SetDNS(windows.AF_INET, servers, []string{})
	}

	// v6 DNS 走 AF_INET6 通道单独设置
	if len(cSess.DNS6) > 0 {
		var servers6 []netip.Addr
		for _, dns := range cSess.DNS6 {
			if addr, e := netip.ParseAddr(dns); e == nil {
				servers6 = append(servers6, addr)
			}
		}
		if len(servers6) > 0 {
			if e := iface.SetDNS(windows.AF_INET6, servers6, []string{}); e != nil {
				base.Error("set ipv6 dns failed:", e)
			}
		}
	}
	return err
}
