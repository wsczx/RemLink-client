// Wails v3 运行时桥接（bundled runtime，对应 Go 中间件 /wails/runtime.js）。
// 等价 wails generate bindings -b 的产物：本机因 cgo 墙无法跑生成器，故手写为同款形态。
// /wails/runtime.js 由 Go 运行时提供，TS 无法静态解析，运行时由 webview 加载。
// @ts-ignore
import { Call, Events } from "/wails/runtime.js";

export { Call, Events };
