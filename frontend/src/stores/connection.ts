import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { ClientService, onEvent } from "../api/client";
import type {
  TunnelInfo,
  TunnelStats,
  ConnStatus,
  AppEvent,
} from "../wailsjs/go/ClientService/models";

export const useConnectionStore = defineStore("connection", () => {
  const status = ref<ConnStatus>("idle");
  const server = ref("");
  const tunnel = ref<TunnelInfo | null>(null);
  const stats = ref<TunnelStats | null>(null);
  const error = ref("");

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
      return;
    }
    stats.value = await ClientService.GetStats();
  }

  onEvent(applyEvent);
  return {
    status,
    server,
    tunnel,
    stats,
    error,
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
