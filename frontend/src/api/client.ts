// 事件桥 + 绑定再导出。
// Go 侧 handleEvent 统一通过 application.Get().Event.Emit("event", e) 派发，
// 前端用 Events.On("event", cb) 订阅；RPC 调用直接走生成的 ClientService 绑定。
import * as ClientService from "../wailsjs/go/ClientService";
import { Events } from "../wailsjs/runtime";
import type { AppEvent } from "../wailsjs/go/ClientService/models";

type Listener = (e: AppEvent) => void;

const listeners = new Set<Listener>();
let unsub: (() => void) | null = null;

// 初始化事件订阅（在 Pinia 装配后、store 注册 onEvent 前调用）。
// 注意：Wails v3 的 Events.On 回调收到的是 WailsEvent 包装对象（{name, data, sender}），
// 真实载荷在 .data 里（部分版本还会包成数组），必须解包后再派发给 store。
export function initEventBridge(): void {
  if (unsub) return;
  unsub = Events.On("event", (ev: any) => {
    let data: AppEvent | undefined =
      ev && typeof ev.data !== "undefined" ? ev.data : ev;
    if (Array.isArray(data)) data = data[0];
    if (!data || typeof data !== "object") return;
    listeners.forEach((l) => {
      try {
        l(data as AppEvent);
      } catch (err) {
        console.error("[remlink] event listener error", err);
      }
    });
  });
}

export function onEvent(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export { ClientService };
