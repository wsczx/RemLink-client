// 类型化模型：对齐 Go 侧 remlink-client/internal/types 的导出结构。
// （本机因 cgo 墙无法跑 wails generate bindings，此处手写为生成器同款形态；
//  FQN 调用与 NewService 自动暴露完全兼容。）

export type ConnStatus =
  | "idle"
  | "connecting"
  | "need_auth"
  | "connected"
  | "reconnecting"
  | "disconnected"
  | "error";

export interface ServerEntry {
  name: string;
  host: string;
  group: string;
  username: string;
}

export interface TunnelInfo {
  ipv4: string;
  ipv6: string;
  dns: string[];
  mtu: number;
  rx_bytes: number;
  tx_bytes: number;
  duration_secs: number;
  server: string;
}

export interface TunnelStats {
  rx_bytes: number;
  tx_bytes: number;
  channel: string;
  cipher: string;
}

export interface PolicySummary {
  prefer_ipv6: boolean;
  compression: boolean;
  fakedns: boolean;
  split_dns: boolean;
  allow_lan: boolean;
  kill_switch: boolean;
  acl_count: number;
  bandwidth?: number;
  bandwidth_up?: number;
  quota?: number;
}

export interface PolicyInfo {
  group?: string;
  policy_name?: string;
  policy: PolicySummary;
  routes_include?: string[];
  routes_exclude?: string[];
  dns?: string[];
  endpoints?: string[];
}

export interface Notice {
  level: string; // info | warn | error
  text: string;
  time: number; // Unix 秒
}

// 设备绑定条目（控制通道 device_list 下推）。
export interface DeviceBinding {
  device_id: string;
  device_name: string;
  platform: string;
  this_device: boolean;
  bound_at: number;
  last_active: number;
}

// 原生认证表单（引擎消费服务端 WebAuth JSON API 后下发）。
export interface WebAuthForm {
  status: string; // select_group | credentials | otp | sms_phone | sms | radius | change_pwd
  groups?: string[];
  hint?: string;
  error?: string;
  username?: string;
  phone_masked?: string;
  challenge_msg?: string;
}

// 引擎 → UI 事件（对齐 types.Event）。
export interface AppEvent {
  kind: string; // status | form | webauth | tunnel | log | error | sso-login | policy | notice | devices
  status?: ConnStatus;
  form?: unknown;
  webauth?: WebAuthForm;
  tunnel?: TunnelInfo;
  message?: string;
  url?: string;
  policy?: PolicyInfo;
  notice?: Notice;
  devices?: DeviceBinding[]; // device_list 下推的已绑定设备
}
