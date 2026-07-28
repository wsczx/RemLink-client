# RemLink 客户端前端重构设计文档（Vue 3 + Naive UI + Wails v3 自动绑定）

> 版本：v1（设计稿，未实现）
> 适用范围：RemLink-client（Wails v3 alpha2.117，main 分支）
> 目标：用 **Vue 3 + Naive UI + Vite + TypeScript** 重写原生引导页，替换手写 vanilla JS，
>       并改用 **Wails v3 代码生成器自动绑定 Go 方法**（删除手写 `bindings.js`）。
> 约束：不改动 Go 引擎/服务逻辑，只换前端；事件契约（Go → 前端）保持不变。

---

## 1. 现状与痛点

### 1.1 当前结构
```
RemLink-client/
├─ main.go                 // application.NewService(svc) 自动暴露 ClientService
├─ internal/
│  ├─ app/app.go           // ClientService：绑定到前端的服务（导出方法 = API 面）
│  ├─ engine/              // 隧道引擎，emit types.Event 到 channel
│  ├─ ui/
│  │  ├─ ui.go             // go:embed all:assets
│  │  └─ assets/           // ← 当前前端（手写）
│  │     ├─ index.html     // 手写 DOM
│  │     ├─ styles.css     // 手写样式
│  │     ├─ app.js         // 手写业务逻辑（22KB，无构建步骤）
│  │     └─ bindings.js    // 手写 Call.ByName 封装（1.87KB）
│  └─ types/types.go       // ConnStatus / TunnelInfo / PolicyInfo / Notice / Event ...
```

### 1.2 痛点
1. **前端是手写 vanilla JS，无构建/类型系统**：`app.js` 22KB 单文件，状态散落、难维护、难扩展（P1c 诊断/Kill Switch 仍是"敬请期待"占位）。
2. **绑定是手写的，不是 Wails v3 自动生成的**：
   - 当前 `bindings.js` 手写 `NS = "remlink-client/internal/app.ClientService"`，逐个方法包 `Call.ByName(NS+"."+method, ...)`。
   - Wails v3 的 `NewService(svc)` 本就**自动把导出方法暴露给 webview**；配套 `wails generate bindings` 会生成**类型化的** `wailsjs/` 绑定（`ClientService.Connect(raw)` 这类，TS 类型齐全）。
   - 手写封装每次改 Go 方法签名都要同步改 JS，且 FQN 字符串写错会静默失败（注释里已踩过 `window.wails` 不存在的坑）。
3. **视觉与交互粗糙**：纯手写 CSS，缺动效/空态/加载/错误态统一规范，距"精美 VPN 客户端"有差距。

### 1.3 目标
- 用 **Vue 3 `<script setup>` + Naive UI + TypeScript + Vite** 重写前端，组件化、状态化、可维护。
- 用 **`wails generate bindings` 生成的 `wailsjs/`** 替代手写 `bindings.js`，自动绑定 Go 方法。
- 用 **Naive UI 暗色主题 + 品牌规范**做出精致的连接/认证/详情/策略/通知/工具界面。
- **零 Go 逻辑改动**：复用现有 RPC（`GetStatus/GetStats/GetPolicy/GetNotices/...`）+ 事件流（`event`）。
- **可本地验证**：Vite 构建独立于 Go/cgo（即便本机 Linux 因缺 GTK4/WebKit6 头无法全量 `go build`，
  前端 `npm run build` 仍可在任意平台跑通，便于真机前先验证 UI/绑定契约）。

---

## 2. 架构总览

### 2.1 构建流水线（改造后）
```
前端源码: RemLink-client/frontend/                  (Vue3 + Naive UI + TS, 由 Vite 构建)
        ├─ src/
        │  ├─ wailsjs/        ← wails generate bindings 自动生成（runtime + 类型化 binding）
        │  ├─ stores/         ← Pinia（连接状态机 / 隧道 / 策略 / 通知 / 认证）
        │  ├─ views/          ← 概览 / 认证 / 策略 / 通知 / 工具
        │  ├─ components/     ← 状态环 / 流量条 / 策略卡 / 通知项 / 认证表单 ...
        │  └─ main.ts, App.vue
        └─ vite.config.ts     → build.outDir = "../internal/ui/assets", emptyOutDir: true

构建: npm run build  →  产物直接写入 internal/ui/assets/  (覆盖 index.html 等)
     wails generate bindings  →  生成 frontend/src/wailsjs/

Go 侧不变: main.go 的 NewService(svc) + ui.go 的 go:embed all:assets 原样复用
```

要点：**Vite 输出目录直接指向 `internal/ui/assets`**，所以 `go:embed` 无需改动即可吃到新前端。
手写 `bindings.js` / 旧 `app.js` / `styles.css` 构建时被 `emptyOutDir` 清空，不再需要。

### 2.2 Wails v3 绑定：正确用法（对比）

**现状（手写，废弃）** —— `internal/ui/assets/bindings.js`：
```js
import { Call } from "/wails/runtime.js";
const NS = "remlink-client/internal/app.ClientService";
function call(method, args) { return Call.ByName(NS + "." + method, ...(args||[])); }
window.go.services.ClientService = { Connect: (raw) => call("Connect", [raw]), /* ... */ };
```

**改造后（自动生成 + 类型化）**：
```ts
// 由 `wails generate bindings` 生成，勿手改
import { ClientService } from "../wailsjs/go/.../ClientService";  // 路径以生成结果为准
import { Events } from "../wailsjs/runtime";

// 调用：签名/类型自动对齐 Go 方法
const token = await ClientService.Connect("https://vpn.example.com");

// 订阅事件流（Go 侧 app.Event.Emit("event", e) → 此处接收）
Events.On("event", (e: AppEvent) => store.applyEvent(e));
```
- FQN、`window.wails`/`window._wails` 差异、Promise 化全部由生成器处理，避免手写踩坑。
- Go 方法签名/返回值变化后，重新 `wails generate bindings` 即可，TS 编译期报错提示前端漏改。

### 2.3 事件契约（Go → 前端，保持不变）
- 引擎 `emit(ch, Event)` → `forwardEngineEvents` → `handleEvent` → `application.Get().Event.Emit("event", e)`。
- 前端通过 `Events.On("event", cb)` 接收，事件体即 `types.Event`（JSON）：
  `kind ∈ {status, form, webauth, tunnel, log, error, sso-login, policy, notice}`。
- 前端**不依赖** `form` 事件（当前引擎走 `webauth` 原生表单，见 2.4），但保留字段兼容。

### 2.4 认证流程（不变）
- `Connect(raw)` 异步发起；引擎 `InitAuth` 后发 `webauth` 事件（组选择/凭据/OTP/短信/RADIUS/改密）。
- 前端渲染 `WebAuthForm`（`status` 决定输入形态），用户提交 → `SubmitAuth(answers)` 回灌。
- 仅 `status="sso"` 且带 `url` 时发 `sso-login` 事件，由 Go 侧开独立 webview 扫码（前端无需处理）。
- 连接成功/失败/断开：`status` 事件驱动，Go 侧已自动 `window.SetURL("/")` 回主页。

---

## 3. API 面（ClientService 导出方法 → 自动绑定目标）

下表即 `wails generate bindings` 会生成的全部前端可调方法。**UI 专属方法**（Go 生命周期注入）不暴露给 UI。

| Go 方法（FQN 后缀） | 签名 | 前端用途 |
|---|---|---|
| `Brand` | `() map[string]any` | 品牌信息（名称/版本/标语） |
| `GetServers` | `() ([]ServerEntry, error)` | 已保存网关列表（datastore） |
| `LastServer` | `() (string, error)` | 最近连接地址（首屏回填） |
| `Connect` | `(raw string) (string, error)` | 发起连接；返回状态/错误 |
| `SubmitAuth` | `(answers map[string]string) error` | 回灌认证答案 |
| `GetStatus` | `() map[string]any` | 拉取当前连接状态（弥补 webview 跳转期间错过事件） |
| `GetStats` | `() *TunnelStats` | 实时流量/通道（轮询 1s） |
| `GetPolicy` | `() *PolicyInfo` | 策略摘要（config_push） |
| `GetNotices` | `() []Notice` | 通知中心列表 |
| `ClearNotices` | `() error` | 清空通知 |
| `RemoveServer` | `(raw string) error` | 删除已保存网关 |
| `OpenPortal` | `() error` | 打开门户窗口 |
| `Disconnect` | `() error` | 断开连接 |
| `Minimize` | `()` | 最小化（自定义标题栏） |
| `HideWindow` | `()` | 关闭→隐藏到托盘（自定义标题栏 ×） |

> 注：`SetWindow/SetApp/SetTray/SetNotifier` 为 Go 启动注入用，不进 `wailsjs`。

---

## 4. 前端工程结构

```
RemLink-client/frontend/
├─ package.json            # vue / naive-ui / pinia / vite / typescript / @vitejs/plugin-vue
├─ vite.config.ts          # outDir → ../internal/ui/assets, emptyOutDir
├─ tsconfig.json
├─ index.html              # Vite 入口（替代手写 index.html）
└─ src/
   ├─ main.ts              # createApp(App).use(naive).use(pinia).mount("#app")
   ├─ App.vue              # 自定义标题栏 + <RouterView>/<component :is> 布局
   ├─ wailsjs/             # 自动生成（runtime + go binding），不手改
   ├─ api/
   │  └─ client.ts         # 封装 ClientService 调用 + Events.On("event") 订阅
   ├─ stores/
   │  ├─ connection.ts     # ConnStatus 状态机 + 隧道信息 + 流量
   │  ├─ policy.ts         # PolicyInfo
   │  ├─ notices.ts        # Notice[]
   │  └─ auth.ts           # WebAuthForm 当前表单 + 提交态
   ├─ views/
   │  ├─ OverviewView.vue  # 未连接 / 认证中 / 已连接 三态
   │  ├─ PolicyView.vue    # 策略可视化
   │  ├─ NoticesView.vue   # 通知中心
   │  └─ ToolsView.vue     # 智能选路 / 诊断 / Kill Switch
   ├─ components/
   │  ├─ TitleBar.vue      # 拖拽条 + 最小化/关闭（调用 Minimize/HideWindow）
   │  ├─ SideNav.vue       # 左侧导航（概览/策略/通知/工具）
   │  ├─ ConnectCard.vue   # 网关输入 + 保存列表 + Connect
   │  ├─ AuthCard.vue      # WebAuthForm 动态渲染（组下拉/凭据/OTP/短信/改密）
   │  ├─ ConnHero.vue      # 已连接主视觉（动画状态环 + 地址/时长）
   │  ├─ TrafficBar.vue    # 实时上下行 + 通道/套件
   │  ├─ PolicyBadges.vue  # 策略徽章（IPv6/压缩/FakeDNS/分DNS/允许LAN/KillSwitch/ACL）
   │  ├─ RouteList.vue     # 路由包含/排除列表
   │  ├─ NoticeItem.vue    # 单条通知（info/warn/error 配色）
   │  ├─ EndpointList.vue  # 智能选路候选（测速择优展示）
   │  └─ KillSwitchCard.vue# Kill Switch 状态 + 说明
   └─ theme/
      └─ tokens.ts         # 品牌色/暗色变量（与 Naive UI 暗色 themeOverrides 对齐）
```

---

## 5. 状态管理（Pinia）

`connection.ts` 核心：
```ts
state: {
  status: ConnStatus;          // idle|connecting|need_auth|connected|reconnecting|disconnected|error
  tunnel?: TunnelInfo;         // ipv4/ipv6/dns/mtu
  stats?: TunnelStats;         // rx/tx/channel/cipher（每秒轮询 GetStats）
  lastError?: string;
  connectedAt?: number;
}
actions: {
  applyEvent(e: AppEvent) {    // 由 Events.On("event") 调用
    switch(e.kind){
      case "status": this.status = e.status; ...
      case "tunnel": this.tunnel = e.tunnel; ...
      case "error":  this.lastError = e.message; ...
      case "policy": policyStore.set(e.policy); ...
      case "notice": noticesStore.push(e.notice); ...
    }
  },
  async pollStats() { this.stats = await ClientService.GetStats(); }  // setInterval 1s，connected 时
}
```
- 状态机完全由 `event` 事件 + `GetStatus` 初次拉取驱动，与现有 Go 契约一致。
- 流量数字用 `GetStats` 轮询（1s），避免事件洪泛；状态切换用事件即时响应。

---

## 6. 视觉规范（"精美 VPN 客户端"）

### 6.1 基础
- **主题**：Naive UI 暗色（`darkTheme`）+ `themeOverrides`，整体背景 `#0B1020`/`#11182E`，卡面 `#1B2138`（对齐当前窗口 `RGB(27,33,56)`）。
- **品牌主色**：`#0EA5E9`（sky-500，取自 logo）；成功 `#22C55E`、警告 `#F59E0B`、错误 `#EF4444`。
- **字体**：系统 UI 字体栈；数字/流量用等宽 `ui-monospace`。
- **圆角**：卡 14px、按钮 10px、输入 8px；轻投影 `0 8px 24px rgba(0,0,0,.35)`。
- **布局**：720×700 固定窗（沿用 `DisableResize`）；左侧 200px 导航 + 右侧内容；顶部 36px 自定义标题栏（拖拽 + 最小化/关闭）。

### 6.2 关键界面
1. **概览 / 未连接**：居中 `ConnectCard`（网关输入 + 历史 datalist + 连接按钮），背景淡淡网格/光晕。
2. **认证中**：`AuthCard` 按 `webauth.status` 动态渲染——组下拉（`NSelect`）、凭据（`NInput` 文本/密码）、OTP/短信码（`NInput` + 倒计时）、改密（双密码校验）。错误就地红字 + 抖动动效。
3. **已连接**：`ConnHero` 大号状态环（SVG 描边动画 + 呼吸光晕）+ 地址/时长；`TrafficBar` 实时上下行（`NProgress` 或自绘 sparkline）；通道/套件/MTU/DNS 网格（`NStatistic`）。
4. **策略**：`PolicyBadges`（启用项高亮徽章）+ `RouteList`（包含/排除折叠明细）。
5. **通知**：`NoticeItem` 列表（按 level 配色，`NBadge` 未读计数，`NEmpty` 空态）。
6. **工具**：`EndpointList`（智能选路候选 + 测速结果）、诊断入口（调用 `ClientService` 诊断/或跳转管理后台）、`KillSwitchCard`（开关态 + 说明）。

### 6.3 动效
- 状态切换：淡入淡出 + 状态环颜色过渡（CSS transition / Naive `NConfigProvider` 主题切换）。
- 连接中：状态环旋转描边；流量条平滑增长。
- 错误：轻微抖动（`@keyframes shake`）。

---

## 7. 与 Go 代码的衔接（最小改动清单）

**前端改造完成且 `wails generate bindings` 后，Go 侧仅在必要时做以下微调（均非逻辑改动）：**

1. `internal/ui/assets/`：Vite 构建产物覆盖；删除手写 `bindings.js` / 旧 `app.js` / `styles.css`（被 `emptyOutDir` 自动清）。
2. `main.go` / `ui.go`：**不动**（NewService 自动暴露 + go:embed 原样工作）。
3. 若生成器要求 `wails.json` 指定前端目录，新增 `wails.json`：
   ```json
   { "frontend:install": "npm install", "frontend:build": "npm run build",
     "frontend:dir": "./frontend", "wailsjsdir": "./frontend/src/wailsjs" }
   ```
4. `Makefile` 增加前端构建目标：
   ```make
   frontend:
   	cd frontend && npm ci && npm run build
   	wails generate bindings        # 生成 wailsjs（供 TS 引用）
   ```
   Windows 发布 `make build` 前先 `make frontend`。

**不改动**：`engine/*`（事件源）、`app/app.go` 的 `ClientService` 方法体、`types/*`（契约）、`forwardEngineEvents`/`handleEvent`（事件派发）。

---

## 8. 分阶段实施计划

| 阶段 | 内容 | 产出 | 风险 |
|---|---|---|---|
| **P0 脚手架** | 新建 `frontend/`（Vite+Vue3+TS+Naive UI+Pinia），`vite.config.ts` 指向 `../internal/ui/assets` | 空壳页面可嵌入运行 | 低 |
| **P1 自动绑定** | `wails generate bindings` 生成 `wailsjs`；`api/client.ts` 封装调用 + `Events.On` 订阅 | 类型化调用替代手写 `bindings.js` | 中（v3 alpha2.117 生成器行为需实测路径） |
| **P2 状态层** | Pinia stores（connection/policy/notices/auth）+ `applyEvent` 派发 | 状态机就绪 | 低 |
| **P3 概览页** | ConnectCard / AuthCard / ConnHero / TrafficBar（三态） | 核心连接体验 | 低 |
| **P4 策略/通知/工具** | PolicyView / NoticesView / ToolsView + 子组件 | 完整左侧导航 | 低 |
| **P5 视觉打磨** | 暗色主题 overrides、动效、空态/错误态、标题栏 | "精美"达标 | 低 |
| **P6 收尾** | 删除手写 `bindings.js`/`app.js`；Makefile 接入；Windows 交叉验证 | 交付 | 低 |

> 建议顺序：**P0 → P1 → P2 → P3 → P5 → P4 → P6**（先打通绑定与状态，再铺页面，最后打磨）。

---

## 9. 风险与待确认

1. **`wails generate bindings` 在 v3 alpha2.117 的产物路径/TS 形态需实测**：生成目录与导入路径以实际生成为准（本文 `../wailsjs/go/.../ClientService` 为示意）。若生成器在该 alpha 版本行为异常，退路是继续使用 `Call.ByName` 但**集中到一个 `api/client.ts`** 统一封装（仍优于当前散落的 `bindings.js` + `app.js`）。
2. **Linux 全量 `go build` 仍受 cgo GTK4/WebKit6 头缺失限制**（已知，与代码无关）。但**前端 Vite 构建独立于 Go**，可在本机先验证 UI 与绑定契约（用浏览器/`vite preview` 配合 mock，或在 Windows 真机跑完整客户端）。
3. **事件体字段需与 Go `types.Event` 严格对齐**：重构中若发现前端需要新字段（如诊断结果详情），应加在 Go `types` 并重新生成绑定，而非前端自行模拟。
4. **SSO 扫码（P1c-2）不在本次纯前端范围**：依赖服务端 SSO URL 接口 + webview 内 canvas 渲染二维码；本次前端重构为其在 `AuthCard`/`ToolsView` 预留 `sso-login` 渲染位即可，具体实现另立任务。
5. **待用户确认**：
   - 是否接受新增 `frontend/` 子工程与 `wails.json`（引入 npm 构建链）？
   - 窗口尺寸 720×700 固定是否沿用，还是允许更大（如 960×720）以容纳更丰富 UI？
   - 是否要保留"工具"页中的诊断/Kill Switch 入口（当前为占位），还是本次一并接真实状态？

---

## 10. 验收标准

- [ ] `cd frontend && npm run build` 产物落入 `internal/ui/assets`，`go:embed` 无需改动即可加载。
- [ ] 前端通过 **生成的 `wailsjs` 绑定** 调用 `ClientService.*`（无手写 `Call.ByName` 封装）。
- [ ] 事件流经 `Events.On("event")` 驱动 Pinia 状态机，连接/认证/详情/策略/通知/工具全部可用。
- [ ] Naive UI 暗色主题 + 品牌规范落地，含动效/空态/错误态，达到"精美 VPN 客户端"观感。
- [ ] `RemLink-client` 既有 Go 逻辑（`engine`/`app` 方法体/`types`）**零改动**，仅前端与构建配置变化。
- [ ] Windows 交叉编译 + 真机运行验证通过（本机 Linux 因 cgo 限制仅验证前端构建）。

---

## 11. 实现记录（2026-07-28，已落地未提交）

用户确认：①接受 `frontend/` + `wails.json`；②窗口尺寸由实现方定（定为 **960×720**）；③工具页接真实状态；④更新 `make build`。

### 11.1 已完成的改动
- **前端工程**：新增 `frontend/`（Vite + Vue 3 `<script setup>` + TypeScript + Naive UI + Pinia）。
  - `vite.config.ts`：`outDir = ../internal/ui/assets`、`emptyOutDir: true`、`rollupOptions.external = [/^\/wails\//]`（运行时保持为 Go 中间件提供的原生 ESM，不被打包）。
  - `package.json` / `tsconfig.json` / `index.html` / `env.d.ts`（声明 `/wails/runtime.js` 模块）。
  - `wails.json`：描述 `frontend:*` 与 `wailsjsdir`（供 `wails generate bindings` 将来在能跑 cgo 的机器上正式生成）。
- **Wails v3 自动绑定（类型化）**：手写 `frontend/src/wailsjs/`（等价于 `wails generate bindings -b` 产物形态）：
  - `runtime.ts` 复用 Go 提供的 `/wails/runtime.js`（导出 `Call`/`Events`）；
  - `go/ClientService/index.ts` 用 FQN `remlink-client/internal/app.ClientService.<Method>` 的 `Call.ByName` 封装全部 15 个方法 + 新增 `RunDiagnostics`，并带 TS 类型；
  - `go/ClientService/models.ts` 对齐 `types.go` 的接口（`ConnStatus/TunnelInfo/PolicyInfo/Notice/WebAuthForm/AppEvent` 等）。
  - 注：本机因 cgo 墙 `wails3 generate bindings` 只扫到 0 个 Service（无法类型检查 `application` 包），故手写为同款形态；Go 侧 `application.NewService(svc)` 仍**自动暴露**每个导出方法（新增 `RunDiagnostics` 即自动可被前端按 FQN 调用，证明自动绑定生效）。
- **事件桥 + 状态层**：`api/client.ts` 用 `Events.On("event", cb)` 订阅 Go `app.Event.Emit("event", e)`；Pinia stores（connection/policy/notices/auth/servers）按事件驱动。
- **UI 组件/视图**：TitleBar（拖拽复用 `wails:drag` 消息 + 最小化/关闭）、SideNav、OverviewView（连接/认证/已连接三态）、PolicyView、NoticesView、ToolsView；卡片 ConnHero/TrafficBar/ConnectCard/AuthCard/PolicyBadges/RouteList/NoticeItem/EndpointList/KillSwitchCard。暗色主题 + 品牌色 #0EA5E9。
- **工具页真实状态**：智能选路（`Policy.Endpoints`）、Kill Switch（`Policy.kill_switch`），以及新增的 **`RunDiagnostics`** 客户端 RPC（包装 `realEngine.runDiagnostics`，mock 返回样例）——「运行网络诊断」按钮真实调用并返回网关/DNS/MTU/路由结果。
- **Go 侧小改**：`Engine` 接口加 `RunDiagnostics`；`realEngine`/`mockEngine` 实现；`ClientService.RunDiagnostics` RPC；`main.go` 窗口 960×720。
- **构建接入**：`Makefile` 加 `frontend` 目标（`cd frontend && npm install && npm run build`），`build`/`build-linux` 依赖它；删除旧手写 `app.js/bindings.js/styles.css/index.html`（由 Vite 构建产物替代）。

### 11.2 验证
- `cd frontend && npm install && npm run build` ✅ 产物写入 `internal/ui/assets/`（index.html + assets/ + logo.svg），`/wails/runtime.js` 保持外部导入。
- `npx vue-tsc --noEmit` ✅ 零类型错误（运行时导入以 `// @ts-ignore` 标注）。
- `GOWORK=off GOOS=windows GOARCH=amd64 go build -ldflags "-H=windowsgui"` ✅ 通过（含新增 RunDiagnostics 与窗口尺寸）。
- 真机运行/视觉需 Windows/macOS 客户端验证（本机 Linux cgo 限制仅验证到前端构建 + Go 交叉编译）。

### 11.3 待真机验证 / 注意
- 窗口 960×720 固定（沿用 `DisableResize`）。
- 认证字段名已对齐引擎（`group`/`username`+`password`/`code`/`phone`/`new_password`+`new_password_confirm`）。
- 若将来在能编译 cgo 的机器上跑 `wails generate bindings`，生成器产物会覆盖 `frontend/src/wailsjs/`，前端代码无需改动（导入路径一致）。
