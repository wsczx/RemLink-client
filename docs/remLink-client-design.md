# RemLink 专属客户端 —— 设计文档 v2

> 目标：把 `RemLink-client` 从「AnyConnect 兼容桩」升级为 **RemLink 专属客户端**。
> 本文档为与用户逐轮对齐后的**权威方案**（v1 的「状态不同步」诊断已吸收，范围按 v2 决策扩展）。
>
> **已确认的范围决策（用户拍板）**：
> - **P0 = 控制通道（状态同步+服务端可控） 与 AnyConnect 平替补全 同批做**（一次到位）。
> - **控制通道机制 = WebSocket 长连**（复用服务端已有的 `gorilla/websocket` + `SyslogHub` 模式）。
> - **专属增量全部纳入**：设备绑定/客户端证书、TAP/macvtap L2、策略/FakeDNS/ACL 可视化、SSO 原生平替；并参考飞连等企业 VPN 客户端自补：断网保护(Kill Switch)、网络诊断、消息通知中心、智能选路/就近接入。
> - **平台范围 = 仅桌面三平台**（Linux/Windows/macOS）；移动端（NetworkExtension/VpnService）列为后续独立硬仗，本期待定。

---

## 1. 定位：RemLink 专属 vs AnyConnect 平替

| 维度 | Cisco AnyConnect（平替基线） | RemLink 专属客户端（本文目标） |
|---|---|---|
| 连接/断开状态准确 | ✅ | ✅（P0 控制通道，秒级） |
| 网关主动断开/踢人 | 部分 | ✅ 服务端可主动踢+发原因 |
| 断线自动重连 | ✅ | ✅（重连失败稳定显示「已断开」） |
| IPv6 双栈 | ✅(v1 已做) | ✅ |
| **服务端↔客户端双向状态** | ❌ | ✅ 控制通道为「在线」唯一真源 |
| **设备信任/绑定+客户端证书** | ❌ | ✅（P1） |
| **TAP/macvtap L2 桥接** | ❌（AnyConnect 不做） | ✅（P1，平台受限） |
| **策略/FakeDNS/ACL 可视化** | ❌ | ✅（P1） |
| **断网保护(Kill Switch/Always-on)** | 企业版有 | ✅（P1，专属企业能力） |
| **网络诊断/自助排障** | ❌ | ✅（P1） |
| **消息通知中心** | ❌ | ✅（P1，服务端下发） |
| **智能选路/就近接入** | ❌ | ✅（P1） |
| **SSO 原生内嵌(企微/飞书/微信扫码)** | 通用浏览器 | ✅（P1，客户端内二维码） |

**原则**：P0 先让客户端"像一个成熟 VPN 客户端且不骗状态"；P1 再叠加 AnyConnect 没有的"专属"能力，形成差异化。

---

## 2. 现状诊断（代码核实，摘要）

- 客户端 `watch()` 在 `CloseChan` 触发后**永远显示「重连中」**，服务端宕机也不显式「已断开」→ 状态欺骗。
- `tls.go` 的 switch 只认 `0x00/0x03/0x04`，**不处理 `0x05 DISCONNECT` / `0x07 KEEPALIVE`** → 服务端经 CSTP 下发断开指令时客户端无响应。
- 服务端 `CloseSess(token, code)` 已存在，但**无任何「管理后台→在线客户端」触发入口**（无 admin kick API、无下发通道）。
- 服务端已有 `gorilla/websocket` + `SyslogHub`（`server/handler/syslog_ws.go`）、会话模型含 `Token`/`UniqueIdGlobal`/`MacAddr`/`LimitTime`/`UserLogoutCode`——控制通道与设备身份的**服务端基础现成**。

---

## 3. 总体架构

```
┌──────────────┐   ① CSTP/TLS + DTLS 数据隧道(业务流量)      ┌──────────────┐
│  RemLink     │   ← DPD/Keepalive(存活兜底, 仅参考) ──→      │  服务端       │
│  Client      │                                            │ (RemLink     │
│  (桌面三平台) │   ② 控制通道 WebSocket(/api/client/ctl)      │  -private)   │
│              │      → 心跳+状态/流量上报+能力声明+ack        │ ClientCtlHub │
│              │      ← disconnect/reauth/config_push/        │ (新增, 复用   │
│              │         message/diag_request/device_challenge│  SyslogHub)   │
│              │   ③ 管理后台(管理员) ──触发命令──→ ClientCtlHub │ admin API    │
│              │   ④ 门户/SSO(企微/飞书/微信) ←→ 客户端原生内嵌 │ /api/admin/*  │
└──────────────┘                                            └──────────────┘
        │
        └─ ⑤ 客户端本地能力层：Kill Switch / 网络诊断 / 通知中心 / 智能选路 / TAP 桥接
```

- **通道①**：承载业务流量；DPD 仅作兜底，**状态判定以②为权威**。
- **通道②（P0 核心）**：经 session-token 认证的 WebSocket，双向；**状态同步唯一真源**。
- **通道③（P0 服务端）**：管理员查看在线客户端、踢人、发消息；经②下发。
- **通道④（P1）**：SSO 原生内嵌，减少通用 webview 依赖。
- **能力层⑤（P1）**：纯客户端侧企业能力，不依赖服务端协议新增（部分需服务端开关）。

---

## 4. P0 —— 控制通道 + AnyConnect 平替（同批交付）

### 4.1 控制通道协议（JSON over WebSocket）
- **端点**：`wss://<host>/api/client/ctl`，握手带 `X-RemLink-Token: <session-token>`（复用 CSTP 签发的 token；服务端 `Token2Sess` 校验）。
- **心跳**：客户端每 **15s** 发 `heartbeat`；服务端 **25s** 未收到则将该客户端从权威在线集合移除（CSTP 仍由 DPD 回收）。
- **统一信封**（字段按需）：

客户端→服务端：
```json
{ "type": "heartbeat", "ts": 1700000000, "status": "connected",
  "channel": "DTLS", "rx": 12345, "tx": 678, "uptime": 3600,
  "caps": ["ipv6","split-dns","compression","kill-switch","tap"] }
{ "type": "ack",        "id": "<cmd-id>" }
{ "type": "config_ack", "ok": true, "err": "" }
{ "type": "diag_result", "id":"...", "results": { "gw_ping": true, "dns": true, "routes": true } }
{ "type": "device_proof", "nonce":"...", "pubkey":"...", "sig":"..." }
```

服务端→客户端（命令）：
```json
{ "type":"disconnect",     "id":"...", "reason":"admin_kick|server_shutdown|session_expired|quota_exceeded" }
{ "type":"reauth",         "id":"...", "reason":"token_expired" }
{ "type":"config_push",    "id":"...",
  "routes_include":["10.0.0.0/255.255.255.0"], "routes_include6":["2001:db8::/32"],
  "routes_exclude":[...], "routes_exclude6":[...],
  "dns":["10.0.0.1"], "dns6":[...],
  "policy": { "prefer_ipv6": false, "compression": true, "split_dns": true,
              "fakedns": true, "kill_switch": false } }
{ "type":"message",        "id":"...", "level":"info|warn|error", "text":"维护通知：今晚 22:00 重启" }
{ "type":"diag_request",   "id":"..." }
{ "type":"device_challenge","id":"...", "nonce":"..." }
```

### 4.2 服务端改造（RemLink-private）
1. **新增 `ClientControlHub`**：照搬 `SyslogHub` 的 `Hub/Client` 模式，key 由 `token` 索引；提供 `Send(token, msg)` / `Broadcast(msg)` / `OnlineCount()`。
2. **`/api/client/ctl` WS 端点**（在 `initRoute` 注册）：升级后校验 token → 注册到 Hub → 启 `readPump`（忽略客户端消息体，仅保活）/ `writePump`（Ping 保活）。
3. **会话生命周期打通**：CSTP 建链成功（`link_cstp.go` 发完配置头后）把 `token` 登记到 `ClientControlHub`；`CloseSess`/`CSess.Close` 时注销。
4. **admin API**（管理后台「在线客户端」）：
   - `GET  /api/admin/clients` → 在线列表（用户/设备/IP/接入时间/流量/通道/caps）。
   - `POST /api/admin/clients/{token}/disconnect` → 经 Hub 下发 `disconnect(admin_kick)`；Hub 下发失败则直接 `CloseSess`。
   - `POST /api/admin/clients/{token}/message` → 经 Hub 下发 `message`。
5. **优雅停机**：`SIGTERM` 处理里对每个在线客户端经 Hub 发 `disconnect(server_shutdown)`，客户端秒级感知（不再等 25s DPD）。

### 4.3 客户端改造（RemLink-client）
1. **新增 `ControlClient`**（`internal/engine/control.go`）：连 `wss://.../api/client/ctl`，带 token；每 15s 发 `heartbeat`（含 `Stats()` + 当前 `status` + `caps`）；读循环分发命令。
2. **状态机以控制通道为权威**：
   - 控制 WS 断开且 1 个心跳周期未重连 → 立即 `emit(disconnected)`，**不再无限「重连中」掩盖**。
   - 收到 `disconnect` → 立即拆 CSTP + `vpnc.ResetRoutes` + 显式「已断开(原因)」。
   - CSTP DPD 超时仅兜底。
3. **`tls.go` 补 `0x05 DISCONNECT` / `0x07 KEEPALIVE`**：与 DTLS 对齐，收到 `0x05` 走 `cSess.Close()`。
4. **重连策略收敛**：连续 N 次（默认 3）重连失败 → 稳定显示「已断开」，由用户或 `reauth` 恢复；避免「一断就闪 connected」假象。
5. **`reauth` 处理**：触发静默 SSO 刷新（优先）/ 否则整条重认证；不丢隧道配置。

### 4.4 P0 验收
- 杀掉服务端进程 / `SIGTERM` → 客户端 **5s 内** 显示「已断开(服务端已停止)」。
- 后台「踢人」→ 客户端秒级断开并显示「已被管理员断开」。
- 断网恢复后自动重连成功；连续失败稳定显示「已断开」而非假「重连中」。

---

## 5. P1 —— RemLink 专属差异化

> 以下按「用户已确认全纳入」+「参考飞连等补强」列出。每项标注服务端/客户端改动与平台注意。

### 5.1 设备绑定 / 客户端证书（专属身份）
- **客户端**：首次连接生成 **Ed25519 设备密钥对**，公钥存 OS 钥匙串（Linux secret-service / Windows DPAPI / macOS keychain）；上报 `UniqueIdGlobal` + 设备指纹（已在 CSTP `cr.DeviceId` 中）。
- **服务端**：复用 portal `/api/devices` 提供「设备注册/信任」；新增「仅信任设备可连」开关（组策略级）；连接时按 `UniqueIdGlobal` 校验信任。
- **客户端证书**（可选增强）：从 portal `/api/certs/download` 拉取用户/设备证书，存钥匙串；TLS 层 `ClientAuth` 出示，服务端 `ClientCAs` 校验。
- **控制通道配合**：服务端不定期发 `device_challenge{nonce}` → 客户端用设备私钥签名回 `device_proof`，作为持续信任证明（防 token 泄露后被冒用）。

### 5.2 TAP / macvtap L2 桥接（真正区别于 AnyConnect）
- **服务端**：在 CSTP 协商阶段按组 `link_mode` 下发 `X-CSTP-Link-Mode: TAP`（或 macvtap）；隧道内改为封装**以太网帧**（`LTypeEthernet`，服务端 `allTapRead/Write` 已支持）。
- **客户端**：
  - Linux：用 `water.TAP` 建 TAP，桥入 `remlink0`（参考服务端 `link_tap.go`），做 ARP/NDP 代理由服务端承担，客户端只需正确收发以太帧。
  - macOS：`utun` 仅 L3；L2 需 `utun` + 桥或第三方驱动，**受限**，v2 先不做 macOS TAP。
  - Windows：**wintun 只支持 L3**；做 TAP 需集成 `tap-windows6` 驱动（独立安装/签名），**重活**，列为 P1 收尾或 P2。
- **验收（Linux 优先）**：组设为 TAP 模式时，客户端拿到的是桥接 L2 接口，能拿到同网段 ARP、互通二层。

### 5.3 策略 / FakeDNS / ACL 可视化（让「专属」可见）
- **服务端**：`config_push` 携带精简策略摘要（`prefer_ipv6`/`compression`/`split_dns`/`fakedns`/`kill_switch`/`acl_mode`）；或首连时随 `heartbeat` 的 `caps` 声明 + 一条 `policy_push`。
- **客户端 UI**：连接详情页新增「策略」卡片——IPv6 偏好、压缩、Split-DNS、FakeDNS 开关、ACL 模式；路由表可视化（包含/排除 v4+v6）；实时流量 + DTLS/TLS 通道（v1 已做）。

### 5.4 SSO 原生平替（企微/飞书/微信扫码）
- **现状**：客户端用通用 webview 打开 WebAuth SPA，靠 `localhost:29786` 回跳拿 token。已可用。
- **P1 增强**：
  - 企微/飞书：客户端内**直接渲染登录二维码**（从服务端 SSO 授权 URL 生成），用户扫码后客户端轮询 `WebAuthContinue` 拿结果，**不再整页 webview**。
  - 微信：复用 `WXAuth/callback` 深链/扫码。
  - 保留 webview 作为降级通道（异常时回退）。
- **价值**：登录体验从「跳浏览器」变为「客户端内一键扫码」，贴合飞连式体验。

### 5.5 断网保护 / Kill Switch（Always-on，高价值企业能力）
- **客户端**（平台防火墙）：
  - Linux：`nftables`/`iptables` 规则——隧道预期 up 时，**仅允许经 tun 接口的流量**，阻断所有明文出网（除到服务端的 VPN 端口）。
  - Windows：WFP（`hns`/filter 引擎）等效规则。
  - macOS：`pf` 锚点规则。
- **开关**：`config_push.policy.kill_switch` 或服务端组策略控制；用户可在客户端开关。
- **价值**：杜绝断连/重连窗口期的流量泄露，是「企业专属」的标志性能力。

### 5.6 网络诊断 / 自助排障
- **服务端**发 `diag_request{id}` → **客户端**执行：网关 Ping、DNS 解析、路由可达性、MTU 探测 → 回 `diag_result`。
- **UI**：连接详情页「诊断」按钮，本地即时自检 + 可上报管理员。
- **价值**：减少「连上了但访问不了」类的工单。

### 5.7 消息通知中心
- **服务端** `message` 命令 → 客户端**系统通知 + 托盘角标 + 通知中心列表**（前端持久化最近 N 条）。
- 配合 P0 的 `server_shutdown`/`admin_kick` 原因，统一走通知中心展示。

### 5.8 智能选路 / 就近接入
- **客户端**：允许多个 `ServerEntry`；连接前对各入口测延迟，自动选最优；服务端可在 `profile` 或 `config_push` 下发候选入口列表。
- **价值**：多活/异地部署时用户体验更顺。

### 5.9 工作台 / 应用门户（复用既有）
- 复用已有 portal「进入门户」；P1 增强为「工作台」快捷入口（内网应用卡片），客户端原生展示。

---

## 6. 实施路线（里程碑）

| 阶段 | 内容 | 涉及仓库 | 验收 |
|---|---|---|---|
| **P0** | 控制通道(WS)+服务端可控(踢/消息/停机)+AnyConnect 平替(0x05/0x07、状态机不掩盖、重连收敛) | RemLink-client + RemLink-private | 服务端停/踢 → 客户端 5s 内断开且原因明确；重连失败稳定「已断开」 |
| **P1a** | 消息通知中心 + 策略/FakeDNS/ACL 可视化 + 智能选路 | 同上 | 通知可达；策略可见；多入口择优 |
| **P1b** | 设备绑定/证书 + `device_challenge` 持续信任 | 同上 | 仅信任设备可连；challenge 通过 |
| **P1c** | SSO 原生内嵌(扫码) + 网络诊断 | RemLink-client 为主 | 客户端内扫码登录；诊断可达 |
| **P1d** | 断网保护(Kill Switch) | RemLink-client | 断连窗口无明文泄露 |
| **P1e** | TAP/macvtap（Linux 优先，Windows 驱动后置） | RemLink-client + RemLink-private | Linux TAP 二层互通 |
| **P3** | 管理后台「在线客户端」UI（踢人/消息/详情/诊断） | RemLink-private 前端 | 管理员可观测管控 |

> 平台顺序建议：Linux → Windows → macOS；TAP 的 macOS/Windows 受限项明确标注、分期。

---

## 7. 风险与开放问题
- **双仓库联调**：P0 起即需同时改 `RemLink-client` 与 `RemLink-private`，在 `dev` 分支联调；老服务端无 `/api/client/ctl` 时客户端降级为旧 CSTP DPD 行为（不影响现有连接）。
- **WS 与 CSTP 一致性**：WS 掉线 ≠ CSTP 必死；**决策：WS 掉线即视为失去管控，客户端主动拆 CSTP 并 disconnect**，避免孤儿隧道（与 v1 一致）。
- **心跳开销**：15s 心跳对千级并发可控；海量客户端需服务端批处理+按需拉取。
- **Windows TAP 驱动**：wintun 仅 L3，TAP 需 `tap-windows6`，涉及驱动签名/安装，工作量与风险高，建议放 P1e 收尾或独立评估。
- **Kill Switch 误伤**：规则需放行 VPN 端口本身与 DNS（如需），避免把自己锁死；提供「紧急关闭」兜底。
- **设备密钥存储安全**：OS 钥匙串为主，明文落盘禁止；私钥不出客户端。

---

## 8. 验证计划
- **单元**：`ControlClient` 心跳/命令分发；`tls.go` 0x05 处理；`device_challenge` 签名校验；Kill Switch 规则增删。
- **集成（dev 联调）**：启服务端→客户端连→后台踢人→秒级断开；`SIGTERM`→5s 内断开；`config_push`→路由/策略热生效；`message`→通知中心；`diag_request`→诊断回传；仅信任设备→非信任设备被拒。
- **回归**：三平台 IPv6 双栈、DTLS 建立、自动重连不受影响；老服务端（无控制通道）下客户端降级正常。
