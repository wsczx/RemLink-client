<template>
  <div class="panel-scroll overview">
    <!-- ── 认证态：居中认证卡片 ── -->
    <div v-if="auth.visible" class="hero-center">
      <AuthCard />
    </div>

    <!-- ── 已连接：环形仪表盘 + 详情 ── -->
    <div v-else-if="conn.connected" class="dash">
      <!-- 左：环形流量 -->
      <div class="gauge-wrap">
        <div class="gauge">
          <svg viewBox="0 0 180 180" class="gauge-svg">
            <circle cx="90" cy="90" r="78" class="g-track" />
            <circle
              cx="90" cy="90" r="78"
              class="g-prog"
              :stroke-dasharray="circumference"
              :stroke-dashoffset="dashOffset"
            />
          </svg>
          <div class="gauge-center">
            <div class="g-dot" :class="{ live: true }"></div>
            <div class="g-status">已连接</div>
            <div class="g-rate">{{ peakRate }}</div>
            <div class="g-rate-sub">实时速率</div>
          </div>
        </div>
        <div class="g-legend">
          <div class="g-leg">
            <span class="g-leg-dot down"></span>
            <span class="g-leg-label">下行</span>
            <span class="g-leg-val">{{ fmtRate(downRate) }}</span>
          </div>
          <div class="g-leg">
            <span class="g-leg-dot up"></span>
            <span class="g-leg-label">上行</span>
            <span class="g-leg-val">{{ fmtRate(upRate) }}</span>
          </div>
        </div>
      </div>

      <!-- 右：详情卡片 -->
      <div class="detail-col">
        <div class="total-row">
          <div class="total-item">
            <div class="total-val">{{ fmtTraffic(stats?.rx_bytes) }}</div>
            <div class="total-label">累计下行</div>
          </div>
          <div class="total-item">
            <div class="total-val">{{ fmtTraffic(stats?.tx_bytes) }}</div>
            <div class="total-label">累计上行</div>
          </div>
          <div class="total-item">
            <div class="total-val">{{ fmtDuration(conn.duration) }}</div>
            <div class="total-label">已连接</div>
          </div>
        </div>

        <div class="info-grid">
          <div class="info-card" v-if="tunnel?.ipv4">
            <div class="info-k">IPv4 地址</div>
            <div class="info-v">{{ tunnel.ipv4 }}</div>
          </div>
          <div class="info-card" v-if="tunnel?.ipv6">
            <div class="info-k">IPv6 地址</div>
            <div class="info-v">{{ tunnel.ipv6 }}</div>
          </div>
          <div class="info-card" v-if="tunnel?.dns?.length">
            <div class="info-k">DNS</div>
            <div class="info-v">{{ tunnel.dns.join(", ") }}</div>
          </div>
          <div class="info-card" v-if="stats?.channel">
            <div class="info-k">数据通道</div>
            <div class="info-v">{{ stats.channel }} · {{ stats.cipher }}</div>
          </div>
          <div class="info-card span2">
            <div class="info-k">接入网关</div>
            <div class="info-v">{{ conn.server || "—" }}</div>
          </div>
        </div>

        <button class="disconnect-btn" @click="conn.disconnect()">
          <span class="db-ico">⏻</span> 断开连接
        </button>
      </div>
    </div>

    <!-- ── 连接中：旋转光环 ── -->
    <div v-else-if="conn.busy" class="hero-center">
      <div class="spin-ring">
        <div class="spin-track"></div>
        <svg class="spin-ico" viewBox="0 0 24 24" width="32" height="32">
          <path d="M12 3v8m5.66-6.16a8 8 0 1 1-11.32 0" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
        </svg>
      </div>
      <p class="spin-status">{{ connectingText }}</p>
      <p class="spin-sub">{{ conn.server || '' }}</p>
      <button class="cancel-btn" @click="conn.disconnect()">取消</button>
    </div>

    <!-- ── 未连接：电源按钮主视觉 ── -->
    <ConnectCard v-else />
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useConnectionStore } from "../stores/connection";
import { useAuthStore } from "../stores/auth";
import ConnectCard from "../components/ConnectCard.vue";
import AuthCard from "../components/AuthCard.vue";

const conn = useConnectionStore();
const auth = useAuthStore();
const stats = computed(() => conn.stats);
const tunnel = computed(() => conn.tunnel);

const connectingText = computed(() => {
  switch (conn.status) {
    case "reconnecting": return "正在重连…";
    case "need_auth": return "等待身份认证…";
    default: return "正在建立安全隧道…";
  }
});

// ── 环形仪表盘 ──
const circumference = 2 * Math.PI * 78;
const RATE_MAX = 25 * 1024 * 1024; // 25 MB/s 为满环基准
const downRate = computed(() => conn.rxRate);
const upRate = computed(() => conn.txRate);
const peakRate = computed(() => fmtRate(Math.max(downRate.value, upRate.value)));
const dashOffset = computed(() => {
  const r = Math.min(1, peakRateValue.value / RATE_MAX);
  return circumference * (1 - r);
});
const peakRateValue = computed(() => Math.max(downRate.value, upRate.value));

function fmtRate(n: number | undefined): string {
  const v = n ?? 0;
  if (v >= 1024 * 1024) return (v / 1024 / 1024).toFixed(1) + " MB/s";
  if (v >= 1024) return (v / 1024).toFixed(1) + " KB/s";
  return Math.round(v) + " B/s";
}
function fmtTraffic(n: number | undefined): string {
  const v = n ?? 0;
  if (v >= 1024 * 1024 * 1024) return (v / 1024 / 1024 / 1024).toFixed(2) + " GB";
  if (v >= 1024 * 1024) return (v / 1024 / 1024).toFixed(1) + " MB";
  if (v >= 1024) return (v / 1024).toFixed(0) + " KB";
  return v + " B";
}
function fmtDuration(s: number): string {
  if (!s) return "0s";
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  if (h) return `${h}h ${m}m`;
  if (m) return `${m}m ${sec}s`;
  return `${sec}s`;
}
</script>

<style scoped>
.overview { display: flex; flex-direction: column; }

.hero-center {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 100%;
}

/* ── 已连接仪表盘 ── */
.dash {
  max-width: 880px;
  width: 100%;
  margin: 0 auto;
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: 26px;
  align-items: center;
  flex: 1;
  padding: 20px 0;
}
.gauge-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 18px;
}
.gauge {
  position: relative;
  width: 220px;
  height: 220px;
}
.gauge-svg { width: 100%; height: 100%; transform: rotate(-90deg); }
.g-track {
  fill: none;
  stroke: rgba(255, 255, 255, 0.07);
  stroke-width: 12;
}
.g-prog {
  fill: none;
  stroke: url(#g);
  stroke: #0ea5e9;
  stroke-width: 12;
  stroke-linecap: round;
  transition: stroke-dashoffset 0.6s ease;
  filter: drop-shadow(0 0 6px rgba(14, 165, 233, 0.5));
}
.gauge-center {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 3px;
}
.g-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: #34d399;
  box-shadow: 0 0 10px rgba(52, 211, 153, 0.7);
  margin-bottom: 4px;
}
.g-dot.live { animation: pulse 1.8s ease-in-out infinite; }
@keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.45} }
.g-status { font-size: 15px; font-weight: 700; color: #f0f9ff; }
.g-rate { font-size: 26px; font-weight: 800; color: #7dd3fc; letter-spacing: -0.5px; }
.g-rate-sub { font-size: 11px; color: #7e90ab; }

.g-legend { display: flex; gap: 22px; }
.g-leg { display: flex; align-items: center; gap: 7px; font-size: 12.5px; }
.g-leg-dot { width: 9px; height: 9px; border-radius: 3px; }
.g-leg-dot.down { background: #38bdf8; }
.g-leg-dot.up { background: #a78bfa; }
.g-leg-label { color: #8aa0bd; }
.g-leg-val { color: #e2e8f0; font-weight: 600; font-variant-numeric: tabular-nums; }

/* ── 详情 ── */
.detail-col { display: flex; flex-direction: column; gap: 16px; }
.total-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}
.total-item {
  background: rgba(30, 41, 71, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 13px;
  padding: 14px;
  text-align: center;
}
.total-val {
  font-size: 19px;
  font-weight: 700;
  color: #f0f9ff;
  font-variant-numeric: tabular-nums;
}
.total-label { font-size: 11.5px; color: #8aa0bd; margin-top: 4px; }

.info-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 11px;
}
.info-card {
  background: rgba(30, 41, 71, 0.55);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 12px;
  padding: 12px 14px;
}
.info-card.span2 { grid-column: span 2; }
.info-k { font-size: 11.5px; color: #7e90ab; margin-bottom: 4px; }
.info-v {
  font-size: 13.5px;
  color: #e2e8f0;
  font-weight: 500;
  font-family: ui-monospace, "SF Mono", monospace;
  word-break: break-all;
}

.disconnect-btn {
  margin-top: 4px;
  padding: 13px;
  border: 1px solid rgba(248, 113, 113, 0.3);
  border-radius: 13px;
  background: rgba(248, 113, 113, 0.1);
  color: #fca5a5;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.18s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}
.disconnect-btn:hover {
  background: rgba(248, 113, 113, 0.18);
  border-color: rgba(248, 113, 113, 0.5);
  color: #fecaca;
}
.db-ico { font-size: 15px; }

/* ── 连接中动画 ── */
.spin-ring {
  position: relative;
  width: 112px;
  height: 112px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fbbf24;
}
.spin-track {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  border: 3px solid rgba(251, 191, 36, 0.12);
  border-top-color: #fbbf24;
  animation: spin 1s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
.spin-ico { position: relative; z-index: 1; }
.spin-status { font-size: 17px; font-weight: 700; margin-top: 14px; color: #f1f5f9; }
.spin-sub { font-size: 12.5px; color: #8595ad; }

.cancel-btn {
  margin-top: 18px;
  padding: 7px 20px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 8px;
  background: transparent;
  color: #94a3b8;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}
.cancel-btn:hover {
  background: rgba(255, 255, 255, 0.06);
  color: #e2e8f0;
  border-color: rgba(255, 255, 255, 0.2);
}
</style>
