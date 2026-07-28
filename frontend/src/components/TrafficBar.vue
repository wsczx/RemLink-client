<template>
  <n-card class="traffic-card">
    <template #header>
      <div class="card-header">
        <span class="card-title">实时流量</span>
        <span class="card-live">
          <span class="live-dot"></span>
          LIVE
        </span>
      </div>
    </template>
    <div class="traffic-body">
      <div class="t-row">
        <div class="t-label"><span class="t-arrow down">↓</span> 下行</div>
        <b class="t-val rx">{{ rx }}</b>
      </div>
      <div class="t-row">
        <div class="t-label"><span class="t-arrow up">↑</span> 上行</div>
        <b class="t-val tx">{{ tx }}</b>
      </div>
      <!-- 底部通道信息 -->
      <div class="t-footer" v-if="conn.stats?.channel">
        <span>{{ conn.stats.channel }}</span>
        <span v-if="conn.stats.cipher" class="cipher">{{ conn.stats.cipher }}</span>
      </div>
    </div>
  </n-card>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { NCard } from "naive-ui";
import { useConnectionStore } from "../stores/connection";
import { formatBytes } from "../utils/format";

const conn = useConnectionStore();
const rx = computed(() => formatBytes(conn.stats?.rx_bytes || 0));
const tx = computed(() => formatBytes(conn.stats?.tx_bytes || 0));
</script>

<style scoped>
.traffic-card {
  border-radius: 16px !important;
  background: rgba(255, 255, 255, 0.03) !important;
  border: 1px solid rgba(255, 255, 255, 0.05) !important;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.25) !important;
}
.card-header {
  display: flex;
  align-items: center;
  gap: 10px;
}
.card-title {
  font-size: 14px;
  font-weight: 700;
  color: #f1f5f9;
}
.card-live {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 9.5px;
  font-weight: 700;
  letter-spacing: 1.2px;
  color: #22c55e;
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(34, 197, 94, 0.1);
  border: 1px solid rgba(34, 197, 94, 0.2);
}
.live-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #22c55e;
  animation: blink 1.8s ease-in-out infinite;
}
@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.traffic-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.t-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
}
.t-label {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 13px;
  color: #8595ad;
  font-weight: 500;
}
.t-arrow {
  font-size: 12px;
  font-weight: 800;
}
.t-arrow.down { color: #38bdf8; }
.t-arrow.up { color: #a78bfa; }

.t-val {
  font-size: 16px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.t-val.rx { color: #38bdf8; }
.t-val.tx { color: #a78bfa; }

.t-footer {
  margin-top: 6px;
  padding-top: 12px;
  display: flex;
  gap: 10px;
  font-size: 11.5px;
  color: #5e6f8c;
  font-family: "SF Mono", ui-monospace, monospace;
}
.t-footer .cipher::before {
  content: '·';
  margin-right: 10px;
  color: #3a4a66;
}
</style>
