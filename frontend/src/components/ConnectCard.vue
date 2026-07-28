<template>
  <div class="connect-hero">
    <!-- 电源按钮：VPN 客户端核心交互 -->
    <button
      class="power-btn"
      :class="{ ready: !!server.trim() }"
      title="连接"
      @click="doConnect"
    >
      <span class="halo"></span>
      <span class="glow"></span>
      <svg viewBox="0 0 24 24" width="42" height="42">
        <path
          d="M12 3v8m5.66-6.16a8 8 0 1 1-11.32 0"
          fill="none"
          stroke="currentColor"
          stroke-width="2.2"
          stroke-linecap="round"
        />
      </svg>
    </button>

    <p class="hero-title">未连接</p>
    <p class="hero-sub">输入网关地址，点击按钮建立安全隧道</p>

    <!-- 地址输入 -->
    <div class="addr-row">
      <n-input
        v-model:value="server"
        size="large"
        round
        placeholder="vpn.example.com:443"
        @keyup.enter="doConnect"
      >
        <template #prefix><span class="addr-ico">◈</span></template>
      </n-input>
    </div>

    <!-- 错误提示 -->
    <n-alert
      v-if="conn.error"
      class="err-alert"
      type="error"
      :title="conn.error"
      closable
      @close="conn.error = ''"
    />

    <!-- 最近连接 -->
    <div v-if="servers.servers.length" class="recent">
      <div class="recent-head">
        <span class="recent-title">最近连接</span>
        <span class="recent-hint">点击快速填入</span>
      </div>
      <div class="recent-list">
        <button
          v-for="s in servers.servers"
          :key="s.host"
          class="recent-item"
          :class="{ on: server === s.host }"
          @click="server = s.host"
        >
          <span class="recent-ico">◈</span>
          <span class="recent-meta">
            <span class="recent-name">{{ s.name }}</span>
            <span class="recent-host">{{ s.host }}</span>
          </span>
          <span
            class="recent-del"
            title="删除记录"
            @click.stop="servers.remove(s.host)"
            >✕</span
          >
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { NInput, NAlert } from "naive-ui";
import { useConnectionStore } from "../stores/connection";
import { useServersStore } from "../stores/servers";

const conn = useConnectionStore();
const servers = useServersStore();
const server = ref("");

onMounted(() => {
  server.value = servers.last || "";
});

async function doConnect() {
  const raw = server.value.trim();
  if (!raw) return;
  await conn.connect(raw);
}
</script>

<style scoped>
.connect-hero {
  flex: 1;
  min-height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
}

/* ── 电源按钮 ── */
.power-btn {
  position: relative;
  width: 120px;
  height: 120px;
  border-radius: 50%;
  border: 1.5px solid rgba(255, 255, 255, 0.09);
  background:
    radial-gradient(circle at 35% 30%, rgba(255, 255, 255, 0.08), transparent 55%),
    linear-gradient(155deg, #1c2742, #141d36);
  color: #3f4f6b;
  cursor: default;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.28s cubic-bezier(0.34, 1.56, 0.64, 1);
  box-shadow:
    0 20px 50px rgba(0, 0, 0, 0.45),
    inset 0 1px 0 rgba(255, 255, 255, 0.06);
}
.power-btn .halo {
  position: absolute;
  inset: -16px;
  border-radius: 50%;
  border: 2px solid transparent;
  transition: all 0.28s ease;
  pointer-events: none;
}
.power-btn .glow {
  position: absolute;
  inset: -20px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(14, 165, 233, 0), rgba(14, 165, 233, 0));
  opacity: 0;
  transition: opacity 0.28s;
  pointer-events: none;
}

/* ready 态：可点击 */
.power-btn.ready {
  cursor: pointer;
  color: #0ea5e9;
  border-color: rgba(14, 165, 233, 0.35);
  box-shadow:
    0 20px 50px rgba(0, 0, 0, 0.45),
    0 0 40px rgba(14, 165, 233, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.07);
}
.power-btn.ready .halo {
  border-color: rgba(14, 165, 233, 0.22);
  box-shadow: 0 0 36px rgba(14, 165, 233, 0.15);
}
.power-btn.ready:hover {
  transform: translateY(-3px) scale(1.05);
  color: #38bdf8;
  border-color: rgba(56, 189, 248, 0.5);
  box-shadow:
    0 26px 60px rgba(0, 0, 0, 0.4),
    0 0 56px rgba(14, 165, 233, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.09);
}
.power-btn.ready:hover .glow {
  opacity: 1;
  background: radial-gradient(circle, rgba(14, 165, 233, 0.12), transparent 70%);
}
.power-btn:active {
  transform: scale(0.96) !important;
}

/* ── 文字 ── */
.hero-title {
  font-size: 20px;
  font-weight: 700;
  margin-top: 24px;
  letter-spacing: 0.6px;
  color: #f1f5f9;
}
.hero-sub {
  font-size: 13px;
  color: #8595ad;
  margin-top: 4px;
}

/* ── 地址输入 ── */
.addr-row {
  width: min(400px, 86%);
  margin-top: 22px;
}
.addr-ico {
  color: #0ea5e9;
  font-size: 14px;
}
.err-alert {
  width: min(400px, 86%);
  margin-top: 14px;
  border-radius: 10px !important;
}

/* ── 最近连接：卡片列表 ── */
.recent {
  margin-top: 26px;
  width: min(420px, 90%);
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.recent-head {
  display: flex;
  align-items: baseline;
  justify-content: center;
  gap: 10px;
}
.recent-title {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 2.5px;
  color: #5e6f8c;
  font-weight: 600;
}
.recent-hint {
  font-size: 10.5px;
  color: #3f4f6b;
}
.recent-list {
  display: flex;
  flex-direction: column;
  gap: 7px;
}
.recent-item {
  display: flex;
  align-items: center;
  gap: 11px;
  width: 100%;
  padding: 10px 12px;
  border-radius: 11px;
  border: 1px solid rgba(255, 255, 255, 0.07);
  background: rgba(255, 255, 255, 0.03);
  cursor: pointer;
  text-align: left;
  transition: all 0.16s ease;
}
.recent-item:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.12);
  transform: translateY(-1px);
}
.recent-item.on {
  background: rgba(14, 165, 233, 0.1);
  border-color: rgba(14, 165, 233, 0.4);
}
.recent-ico {
  width: 28px;
  height: 28px;
  flex: none;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: rgba(14, 165, 233, 0.12);
  color: #38bdf8;
  font-size: 13px;
}
.recent-item.on .recent-ico {
  background: rgba(14, 165, 233, 0.22);
}
.recent-meta {
  display: flex;
  flex-direction: column;
  min-width: 0;
  flex: 1;
}
.recent-name {
  font-size: 13.5px;
  font-weight: 600;
  color: #dbe4f0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.recent-host {
  font-size: 11.5px;
  color: #8595ad;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: "SF Mono", ui-monospace, monospace;
}
.recent-del {
  flex: none;
  width: 22px;
  height: 22px;
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #475569;
  font-size: 12px;
  opacity: 0;
  transition: all 0.14s;
}
.recent-item:hover .recent-del {
  opacity: 1;
}
.recent-del:hover {
  color: #f87171;
  background: rgba(248, 113, 113, 0.15);
}
</style>
