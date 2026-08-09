import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { ClientService, onEvent } from "../api/client";
import type {
  TunnelInfo,
  TunnelStats,
  ConnStatus,
  AppEvent,
  DeviceBinding,
} from "../wailsjs/go/ClientService/models";

export const useConnectionStore = defineStore("connection", () => {
  const status = ref<ConnStatus>("idle");
  const server = ref("");
  const tunnel = ref<TunnelInfo | null>(null);
  const stats = ref<TunnelStats | null>(null);
  const error = ref("");
  const devices = ref<DeviceBinding[]>([]); // G3：当前账号已绑定设备（设备管理视图）
  const rxRate = ref(0); // 实时下行速率（字节/秒，由两次采样差计算）
  const txRate = ref(0); // 实时上行速率

  const connectedAt = ref(0);
  const tick = ref(0); // 每秒自增，驱动时长重算

  const connected = computed(() => status.value === "connected");
  const busy = computed(() =>
    ["connecting", "reconnecting", "need_auth"].includes(status.value),
  );
  const duration = computed(() => {
    tick.value; // 依赖 tick 触发重算
    return connectedAt.value
      ? Math.floor((Date.now() - connectedAt.value) / 1000)
      : 0;
  });

  function applyEvent(e: AppEvent) {
    if (e.kind === "status" && e.status) {
      status.value = e.status;
      if (e.status === "connected") {
        connectedAt.value = Date.now();
        error.value = "";
      }
      if (e.status === "disconnected" || e.status === "error") {
        connectedAt.value = 0;
      }
    }
    if (e.kind === "tunnel" && e.tunnel) tunnel.value = e.tunnel;
    if (e.kind === "devices" && e.devices) devices.value = e.devices;
    if (e.kind === "error") error.value = e.message || "连接错误";
  }

  async function load() {
    const st = (await ClientService.GetStatus()) as Record<string, any>;
    if (st.status) status.value = st.status as ConnStatus;
    server.value = (st.server as string) || "";
    if (st.tunnel) tunnel.value = st.tunnel as TunnelInfo;
    if (st.error) error.value = st.error as string;
    if (
      st.status === "connected" &&
      typeof st.duration_secs === "number" &&
      st.duration_secs > 0
    ) {
      connectedAt.value = Date.now() - (st.duration_secs as number) * 1000;
    }
  }

  async function connect(raw: string) {
    error.value = "";
    await ClientService.Connect(raw);
  }

  async function disconnect() {
    await ClientService.Disconnect();
  }

  async function pollStats() {
    tick.value++;
    if (!connected.value) {
      stats.value = null;
      rxRate.value = 0;
      txRate.value = 0;
      return;
    }
    const prev = stats.value;
    const next = await ClientService.GetStats();
    stats.value = next;
    if (prev && next) {
      // 速率 = 累计字节差 / 采样间隔（pollStats 约每 4 秒一次）
      const dt = 4;
      const dr = (next.rx_bytes ?? 0) - (prev.rx_bytes ?? 0);
      const ur = (next.tx_bytes ?? 0) - (prev.tx_bytes ?? 0);
      rxRate.value = dr > 0 ? Math.round(dr / dt) : 0;
      txRate.value = ur > 0 ? Math.round(ur / dt) : 0;
    }
  }

  onEvent(applyEvent);
  return {
    status,
    server,
    tunnel,
    stats,
    error,
    devices,
    rxRate,
    txRate,
    connected,
    busy,
    duration,
    applyEvent,
    load,
    connect,
    disconnect,
    pollStats,
  };
});
