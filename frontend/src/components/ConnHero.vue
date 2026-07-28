<template>
  <n-card class="detail-card">
    <!-- 顶部状态条 -->
    <div class="conn-header">
      <div class="conn-left">
        <div class="conn-ring">
          <svg viewBox="0 0 24 24" width="22" height="22">
            <path
              d="M20 6L9 17l-5-5"
              fill="none"
              stroke="currentColor"
              stroke-width="2.6"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
        </div>
        <div class="conn-meta">
          <div class="conn-title">已安全连接</div>
          <div class="conn-host">{{ conn.server }}</div>
        </div>
      </div>

      <div class="conn-right">
        <div class="dur-box">
          <span class="dur-num">{{ formatDuration(conn.duration) }}</span>
          <span class="dur-label">时长</span>
        </div>
      </div>
    </div>

    <!-- 网络信息网格 -->
    <div class="info-grid">
      <div class="info-cell">
        <span class="info-label">IPv4</span>
        <span class="info-val mono">{{ tunnel?.ipv4 || '—' }}</span>
      </div>
      <div class="info-cell" v-if="tunnel?.ipv6">
        <span class="info-label">IPv6</span>
        <span class="info-val mono">{{ tunnel.ipv6 }}</span>
      </div>
      <div class="info-cell">
        <span class="info-label">DNS</span>
        <span class="info-val mono">{{ dnsText }}</span>
      </div>
      <div class="info-cell">
        <span class="info-label">MTU</span>
        <span class="info-val mono">{{ tunnel?.mtu || '—' }}</span>
      </div>
      <div class="info-cell wide" v-if="conn.stats?.channel">
        <span class="info-label">通道</span>
        <span class="info-val mono">{{ conn.stats.channel }}<span v-if="conn.stats.cipher"> / {{ conn.stats.cipher }}</span></span>
      </div>
    </div>

    <!-- 操作按钮（关键：直接放在卡片内，不再依赖外部 footer） -->
    <div class="conn-actions">
      <button class="act-btn primary" @click="openPortal">
        <svg viewBox="0 0 18 18" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M7 3h8v8M14 4L7 11M3 15h6v-6"/></svg>
        进入门户
      </button>
      <button class="act-btn danger" @click="doDisconnect">
        <svg viewBox="0 0 18 18" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M5 12l5-5 5 5M2 10a5 5 0 0 1 5-5h4a5 5 0 0 1 5 5v1a5 5 0 0 1-5 5H7a5 5 0 0 1-5-5v-1z"/></svg>
        断开连接
      </button>
    </div>
  </n-card>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { NCard } from "naive-ui";
import { useConnectionStore } from "../stores/connection";
import { ClientService } from "../api/client";
import { formatDuration } from "../utils/format";

const conn = useConnectionStore();
const tunnel = computed(() => conn.tunnel);
const dnsText = computed(() =>
  tunnel.value?.dns && tunnel.value.dns.length
    ? tunnel.value.dns.join(", ")
    : "—",
);

function openPortal() {
  void ClientService.OpenPortal();
}
async function doDisconnect() {
  await conn.disconnect();
}
</script>

<style scoped>
.detail-card {
  border-radius: 16px !important;
  background: rgba(255, 255, 255, 0.03) !important;
  border: 1px solid rgba(255, 255, 255, 0.05) !important;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.25) !important;
}

/* ── 顶部状态条 ── */
.conn-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}
.conn-left {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}
.conn-ring {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #22c55e;
  border: 2px solid rgba(34, 197, 94, 0.45);
  background: radial-gradient(circle at 50% 40%, rgba(34, 197, 94, 0.16), transparent 70%);
  box-shadow:
    0 0 28px rgba(34, 197, 94, 0.22),
    inset 0 0 12px rgba(34, 197, 94, 0.08);
  animation: breathe 2.6s ease-in-out infinite;
  flex: none;
}
@keyframes breathe {
  0%, 100% { box-shadow: 0 0 28px rgba(34, 197, 94, 0.22), inset 0 0 12px rgba(34, 197, 94, 0.08); }
  50% { box-shadow: 0 0 38px rgba(34, 197, 94, 0.35), inset 0 0 16px rgba(34, 197, 94, 0.12); }
}
.conn-meta {
  min-width: 0;
}
.conn-title {
  font-size: 17px;
  font-weight: 700;
  color: #f1f5f9;
  letter-spacing: 0.2px;
}
.conn-host {
  font-size: 12.5px;
  color: #8595ad;
  margin-top: 3px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ── 右侧时长 ── */
.conn-right {
  flex: none;
}
.dur-box {
  text-align: right;
  padding: 6px 14px;
  border-radius: 10px;
  background: rgba(34, 197, 94, 0.07);
  border: 1px solid rgba(34, 197, 94, 0.14);
}
.dur-num {
  display: block;
  font-size: 18px;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  color: #22c55e;
  line-height: 1.1;
}
.dur-label {
  font-size: 10px;
  color: #4ade80;
  letter-spacing: 0.5px;
  margin-top: 2px;
}

/* ── 信息网格 ── */
.info-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px 16px;
  padding: 16px 0;
  margin-bottom: 18px;
}
.info-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.info-cell.wide {
  grid-column: span 2;
}
.info-label {
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 1px;
  color: #5e6f8c;
  font-weight: 600;
}
.info-val {
  font-size: 13px;
  color: #dbe4f0;
  word-break: break-all;
}
.info-val.mono {
  font-variant-numeric: tabular-nums;
  font-family: "SF Mono", "Cascadia Code", "JetBrains Mono", ui-monospace, monospace;
  font-size: 12.5px;
}

/* ── 操作按钮 ── */
.conn-actions {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}
.act-btn {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 9px 18px;
  border: none;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.18s ease;
  letter-spacing: 0.2px;
}
.act-btn.primary {
  background: linear-gradient(135deg, #0ea5e9, #0284c7);
  color: #fff;
  box-shadow: 0 4px 14px rgba(14, 165, 233, 0.35);
}
.act-btn.primary:hover {
  background: linear-gradient(135deg, #38bdf8, #0ea5e9);
  box-shadow: 0 6px 20px rgba(14, 165, 233, 0.45);
  transform: translateY(-1px);
}
.act-btn.primary:active {
  transform: translateY(0);
}
.act-btn.danger {
  background: rgba(248, 113, 113, 0.1);
  color: #f87171;
  border: 1px solid rgba(248, 113, 113, 0.25);
}
.act-btn.danger:hover {
  background: rgba(248, 113, 113, 0.18);
  border-color: rgba(248, 113, 113, 0.4);
  box-shadow: 0 4px 14px rgba(248, 113, 113, 0.15);
}
</style>
