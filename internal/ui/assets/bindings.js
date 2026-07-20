// 自包含 Wails v3 绑定桥接（嵌入式原生前端，无 wails 代码生成器）。
//
// Wails v3 (alpha2.117) 的运行时 /wails/runtime.js 是一个 ES module，
// 通过 `export { Call }` 暴露 Call，调用约定为：
//   Call.ByName("<package>.<Type>.<Method>", ...args) -> CancellablePromise
// 该版本【没有】 window.wails 这个全局对象（只有 window._wails 作为底层传输），
// 因此必须 import Call，绝不能写 window.wails.Call.ByName（会直接抛 TypeError，
// 导致所有服务调用静默失败、WebView 永远不向服务器发请求）。
// FQN 必须与 Go 侧一致：<packagePath>.<typeName>.<methodName>
//   = remlink-client/internal/app.ClientService.<Method>
// 见 wails v3 pkg/application/bindings.go 的 getMethods()。

import { Call } from "/wails/runtime.js";

const NS = "remlink-client/internal/app.ClientService";

function call(method, args) {
  // Call.ByName(name, ...args) -> CancellablePromise（可被 await）
  return Call.ByName(NS + "." + method, ...(args || []));
}

window.go = window.go || {};
window.go.services = window.go.services || {};
window.go.services.ClientService = {
  Brand:          () => call("Brand", []),
  GetServers:     () => call("GetServers", []),
  LastServer:     () => call("LastServer", []),
  Connect:        (raw) => call("Connect", [raw]),
  GetStatus:      () => call("GetStatus", []),
  OpenPortal:     () => call("OpenPortal", []),
  RemoveServer:   (host) => call("RemoveServer", [host]),
  Disconnect:     () => call("Disconnect", []),
  Minimize:       () => call("Minimize", []),
  HideWindow:     () => call("HideWindow", []),
};
