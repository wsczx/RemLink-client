<template>
  <section class="view devices">
    <header class="view-head">
      <div>
        <h1 class="view-title">设备管理</h1>
        <p class="view-sub">本账号已绑定的可信设备，可在新设备超限时识别并解绑</p>
      </div>
      <n-tag v-if="connected" type="success" :bordered="false" round>已连接</n-tag>
      <n-tag v-else type="warning" :bordered="false" round>未连接</n-tag>
    </header>

    <div v-if="devices.length" class="dev-list">
      <article
        v-for="d in devices"
        :key="d.device_id"
        class="dev-card"
        :class="{ self: d.this_device }"
      >
        <div class="dev-ico" :class="platformClass(d.platform)">
          <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="4" width="14" height="11" rx="1.5"/><path d="M7 19h6M9 15v4"/><rect x="18" y="8" width="4" height="9" rx="1"/></svg>
        </div>
        <div class="dev-meta">
          <div class="dev-name">
            {{ d.device_name }}
            <span v-if="d.this_device" class="dev-self">本机</span>
          </div>
          <div class="dev-sub">
            {{ platformLabel(d.platform) }} · 绑定于 {{ formatTime(d.bound_at) }}
          </div>
          <div class="dev-id">指纹 {{ d.device_id }}</div>
        </div>
        <div class="dev-actions">
          <n-button
            v-if="!d.this_device"
            size="small"
            tertiary
            type="error"
            :disabled="!connected"
            @click="unbind(d)"
          >
            解绑
          </n-button>
          <span v-else class="dev-online">在线</span>
        </div>
      </article>
    </div>

    <div v-else class="dev-empty">
      <div class="dev-empty-ico">
        <svg viewBox="0 0 24 24" width="40" height="40" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="4" width="14" height="11" rx="1.5"/><path d="M7 19h6M9 15v4"/><rect x="18" y="8" width="4" height="9" rx="1"/></svg>
      </div>
      <p>暂无设备信息</p>
      <span class="dev-empty-hint">连接成功后，本机会自动注册为可信设备并在此列出</span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { NButton, NTag, useMessage } from "naive-ui";
import { useConnectionStore } from "../stores/connection";
import type { DeviceBinding } from "../wailsjs/go/ClientService/models";

const conn = useConnectionStore();
const message = useMessage();
const devices = computed(() => conn.devices);
const connected = computed(() => conn.connected);

function platformLabel(p: string) {
  switch (p) {
    case "darwin":
      return "macOS";
    case "windows":
      return "Windows";
    case "linux":
      return "Linux";
    case "android":
      return "Android";
    case "ios":
      return "iOS";
    default:
      return "未知平台";
  }
}
function platformClass(p: string) {
  return "pf-" + (p || "unknown");
}
function formatTime(ts: number) {
  if (!ts) return "—";
  const d = new Date(ts * 1000);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(
    d.getDate(),
  ).padStart(2, "0")}`;
}

// 解绑：当前由管理员在控制台操作；客户端引导联系管理员（服务端解绑 API 后续接入）。
function unbind(d: DeviceBinding) {
  message.info(`「${d.device_name}」解绑需联系管理员在安全控制台操作`);
}
</script>

<style scoped>
.view {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}
.view-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 22px;
}
.view-title {
  font-size: 22px;
  font-weight: 700;
  margin: 0;
  color: #f0f9ff;
}
.view-sub {
  margin: 6px 0 0;
  font-size: 13px;
  color: #8aa0bd;
}

.dev-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-width: 640px;
}
.dev-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  border-radius: 15px;
  background: linear-gradient(160deg, rgba(30, 41, 71, 0.9), rgba(22, 31, 56, 0.9));
  border: 1px solid rgba(255, 255, 255, 0.07);
  transition: all 0.2s ease;
}
.dev-card.self {
  border-color: rgba(14, 165, 233, 0.45);
  box-shadow: inset 0 0 0 1px rgba(14, 165, 233, 0.15);
}
.dev-ico {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #cfe8ff;
  background: rgba(14, 165, 233, 0.12);
}
.dev-meta {
  flex: 1;
  min-width: 0;
}
.dev-name {
  font-size: 14.5px;
  font-weight: 600;
  color: #eef3fb;
  display: flex;
  align-items: center;
  gap: 8px;
}
.dev-self {
  font-size: 10.5px;
  font-weight: 700;
  padding: 1px 7px;
  border-radius: 999px;
  background: rgba(14, 165, 233, 0.2);
  color: #7dd3fc;
}
.dev-sub {
  font-size: 12px;
  color: #8aa0bd;
  margin-top: 3px;
}
.dev-id {
  font-size: 11px;
  color: #5b6b85;
  margin-top: 2px;
  font-family: ui-monospace, monospace;
}
.dev-actions {
  flex: none;
}
.dev-online {
  font-size: 12px;
  font-weight: 600;
  color: #34d399;
}

.dev-empty {
  margin: auto;
  text-align: center;
  color: #8aa0bd;
}
.dev-empty-ico {
  color: #475569;
  margin-bottom: 10px;
}
.dev-empty-hint {
  display: block;
  margin-top: 6px;
  font-size: 12px;
  color: #5b6b85;
  max-width: 300px;
}
</style>
