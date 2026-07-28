<template>
  <div class="panel-scroll overview">
    <!-- ── 认证态：居中认证卡片 ── -->
    <div v-if="auth.visible" class="hero-center">
      <AuthCard />
    </div>

    <!-- ── 已连接：状态仪表盘 + 流量 ── -->
    <div v-else-if="conn.connected" class="connected-col">
      <ConnHero />
      <TrafficBar />
    </div>

    <!-- ── 连接中：旋转光环 ── -->
    <div v-else-if="conn.busy" class="hero-center">
      <div class="spin-ring">
        <div class="spin-track"></div>
        <svg class="spin-ico" viewBox="0 0 24 24" width="32" height="32">
          <path
            d="M12 3v8m5.66-6.16a8 8 0 1 1-11.32 0"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
          />
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
import ConnHero from "../components/ConnHero.vue";
import TrafficBar from "../components/TrafficBar.vue";

const conn = useConnectionStore();
const auth = useAuthStore();

const connectingText = computed(() => {
  switch (conn.status) {
    case "reconnecting": return "正在重连…";
    case "need_auth": return "等待身份认证…";
    default: return "正在建立安全隧道…";
  }
});
</script>

<style scoped>
.overview {
  display: flex;
  flex-direction: column;
}

/* 居中容器（认证 / 连接中 / 未连接共用） */
.hero-center {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 100%;
}

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

.spin-status {
  font-size: 17px;
  font-weight: 700;
  margin-top: 14px;
  color: #f1f5f9;
}
.spin-sub {
  font-size: 12.5px;
  color: #8595ad;
}

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

/* ── 已连接列 ── */
.connected-col {
  max-width: 640px;
  width: 100%;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
</style>
