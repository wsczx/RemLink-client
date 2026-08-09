// ClientService 绑定（类型化，等价 wails generate bindings 产物）。
// Go 侧 application.NewService(svc) 自动暴露 ClientService 的全部导出方法，
// FQN 与手写 bindings.js 一致；此处仅补充 TypeScript 类型。
import { Call } from "../../runtime";
import type {
  ServerEntry,
  TunnelStats,
  PolicyInfo,
  Notice,
  AppEvent,
} from "./models";

const NS = "remlink-client/internal/app.ClientService";

const call = (method: string, args: unknown[] = []): Promise<any> =>
  (Call as any).ByName(NS + "." + method, ...args);

export const Brand = (): Promise<Record<string, any>> => call("Brand");
export const GetServers = (): Promise<ServerEntry[]> => call("GetServers");
export const LastServer = (): Promise<string> => call("LastServer");
export const Connect = (raw: string): Promise<string> => call("Connect", [raw]);
export const SubmitAuth = (
  answers: Record<string, string>,
): Promise<void> => call("SubmitAuth", [answers]);
export const GetStatus = (): Promise<Record<string, any>> => call("GetStatus");
export const GetStats = (): Promise<TunnelStats | null> => call("GetStats");
export const GetPolicy = (): Promise<PolicyInfo | null> => call("GetPolicy");
export const GetNotices = (): Promise<Notice[]> => call("GetNotices");
export const ClearNotices = (): Promise<void> => call("ClearNotices");
export const RemoveServer = (raw: string): Promise<void> =>
  call("RemoveServer", [raw]);
export const OpenPortal = (): Promise<void> => call("OpenPortal");
export const WebVpnMyApps = (): Promise<
  Array<{ name: string; note: string; url: string }>
> => call("WebVpnMyApps");
export const Disconnect = (): Promise<void> => call("Disconnect");
export const Minimize = (): Promise<void> => call("Minimize");
export const HideWindow = (): Promise<void> => call("HideWindow");
export const RunDiagnostics = (): Promise<Record<string, any>> =>
  call("RunDiagnostics");

export type { AppEvent };
