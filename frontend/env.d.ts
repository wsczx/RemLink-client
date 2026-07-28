/// <reference types="vite/client" />

// Wails v3 运行时由 Go 中间件在 /wails/runtime.js 提供（bundled runtime）。
// 以原生 ESM 外部导入，此处声明其类型（等价 wails generate bindings -b 产物）。
declare module "/wails/runtime.js" {
  export interface Call {
    ByName(name: string, ...args: any[]): Promise<any>;
    ByID(id: string, ...args: any[]): Promise<any>;
  }
  export const Call: Call;

  export type EventUnsubscriber = () => void;
  export interface Events {
    On(name: string, callback: (data: any) => void): EventUnsubscriber;
    Once(name: string, callback: (data: any) => void): EventUnsubscriber;
    Off(name: string, callback: (data: any) => void): void;
    Emit(name: string, data?: any): void;
  }
  export const Events: Events;

  export const Log: any;
  export const Window: any;
}
