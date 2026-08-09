<template>
  <section class="view webapps">
    <header class="view-head">
      <div>
        <h1 class="view-title">内网应用</h1>
        <p class="view-sub">通过零信任网关免客户端直接访问的内网 Web 应用</p>
      </div>
      <n-button quaternary size="small" :loading="store.loading" @click="store.refresh()">
        刷新
      </n-button>
    </header>

    <div v-if="store.apps.length" class="app-grid">
      <article
        v-for="app in store.apps"
        :key="app.url"
        class="app-card"
        @click="store.open(app)"
      >
        <div class="app-logo">{{ initials(app.name) }}</div>
        <div class="app-meta">
          <div class="app-name">{{ app.name }}</div>
          <div class="app-note">{{ app.note || "无描述" }}</div>
        </div>
        <div class="app-go">打开 →</div>
      </article>
    </div>

    <div v-else-if="store.loading" class="app-empty">正在加载应用…</div>
    <div v-else class="app-empty">
      <div class="app-empty-ico">🗂️</div>
      <p>暂无可用应用</p>
      <span class="app-empty-hint">如需开通，请联系管理员分配 WebVPN 权限</span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted } from "vue";
import { NButton } from "naive-ui";
import { useWebAppsStore } from "../stores/webapps";

const store = useWebAppsStore();

function initials(name: string) {
  const t = (name || "?").trim();
  if (!t) return "?";
  const parts = t.split(/\s+/);
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
  return t.slice(0, 2).toUpperCase();
}

onMounted(() => store.load());
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
  letter-spacing: 0.3px;
}
.view-sub {
  margin: 6px 0 0;
  font-size: 13px;
  color: #8aa0bd;
}

.app-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(248px, 1fr));
  gap: 14px;
}
.app-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px;
  border-radius: 16px;
  background: linear-gradient(160deg, rgba(30, 41, 71, 0.9), rgba(22, 31, 56, 0.9));
  border: 1px solid rgba(255, 255, 255, 0.07);
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
  overflow: hidden;
}
.app-card:hover {
  transform: translateY(-3px);
  border-color: rgba(14, 165, 233, 0.4);
  box-shadow: 0 12px 34px rgba(2, 12, 27, 0.5);
}
.app-logo {
  width: 46px;
  height: 46px;
  border-radius: 13px;
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 16px;
  color: #f0f9ff;
  background: linear-gradient(135deg, #0ea5e9, #6366f1);
  box-shadow: 0 6px 18px rgba(14, 165, 233, 0.3);
}
.app-meta {
  flex: 1;
  min-width: 0;
}
.app-name {
  font-size: 14.5px;
  font-weight: 600;
  color: #eef3fb;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.app-note {
  font-size: 12px;
  color: #8aa0bd;
  margin-top: 3px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.app-go {
  font-size: 12px;
  font-weight: 600;
  color: #38bdf8;
  opacity: 0;
  transition: opacity 0.2s ease;
  flex: none;
}
.app-card:hover .app-go {
  opacity: 1;
}

.app-empty {
  margin: auto;
  text-align: center;
  color: #8aa0bd;
  font-size: 14px;
}
.app-empty-ico {
  font-size: 40px;
  margin-bottom: 10px;
}
.app-empty-hint {
  display: block;
  margin-top: 6px;
  font-size: 12px;
  color: #5b6b85;
}
</style>
