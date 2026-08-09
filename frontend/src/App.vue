<template>
  <n-config-provider :theme="naiveTheme" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <div class="app-shell">
          <div class="app-body">
            <SideNav v-model:active="active" />
            <main class="app-main">
              <TitleBar />
              <OverviewView v-if="active === 'overview'" />
              <WebAppsView v-else-if="active === 'webapps'" />
              <DevicesView v-else-if="active === 'devices'" />
              <PolicyView v-else-if="active === 'policy'" />
              <NoticesView v-else-if="active === 'notice'" />
              <ToolsView v-else-if="active === 'tools'" />
            </main>
          </div>
        </div>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { NConfigProvider, NMessageProvider, NDialogProvider } from "naive-ui";
import TitleBar from "./components/TitleBar.vue";
import SideNav from "./components/SideNav.vue";
import OverviewView from "./views/OverviewView.vue";
import WebAppsView from "./views/WebAppsView.vue";
import DevicesView from "./views/DevicesView.vue";
import PolicyView from "./views/PolicyView.vue";
import NoticesView from "./views/NoticesView.vue";
import ToolsView from "./views/ToolsView.vue";
import { naiveTheme, themeOverrides } from "./theme/tokens";
import { useConnectionStore } from "./stores/connection";

const active = ref("overview");
const conn = useConnectionStore();
let timer: number | undefined;

onMounted(() => {
  timer = window.setInterval(() => {
    void conn.pollStats();
  }, 1000);
});
onUnmounted(() => {
  if (timer) window.clearInterval(timer);
});
</script>

<style>
:root {
  color-scheme: dark;
}
html,
body,
#app {
  height: 100%;
  margin: 0;
}
body {
  font-family:
    -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto,
    "PingFang SC", "Microsoft YaHei", sans-serif;
  /* 明亮化分层深色：侧栏深、主区亮，避免黑黢黢一片 */
  background:
    radial-gradient(ellipse 900px 500px at 90% -5%, rgba(14, 165, 233, 0.12), transparent 60%),
    radial-gradient(ellipse 700px 450px at -5% 105%, rgba(99, 102, 241, 0.08), transparent 60%),
    #0c1326;
  color: #eef3fb;
  overflow: hidden;
  -webkit-font-smoothing: antialiased;
}

/* ── 视图滚动容器（各页面共用） ── */
.panel-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 28px 32px;
  scrollbar-width: thin;
  scrollbar-color: rgba(148, 163, 184, 0.15) transparent;
}
.panel-scroll::-webkit-scrollbar {
  width: 6px;
}
.panel-scroll::-webkit-scrollbar-track {
  background: transparent;
}
.panel-scroll::-webkit-scrollbar-thumb {
  background: rgba(148, 163, 184, 0.12);
  border-radius: 999px;
}
.panel-scroll::-webkit-scrollbar-thumb:hover {
  background: rgba(148, 163, 184, 0.22);
}

/* ── 页头（策略/通知/工具页） ── */
.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18px;
}
.page-head h2 {
  font-size: 15px;
  font-weight: 700;
  margin: 0;
  letter-spacing: 0.3px;
  color: #f1f5f9;
}

/* ── 卡片统一质感（从主区浮出） ── */
.panel-scroll .n-card {
  border: 1px solid rgba(255, 255, 255, 0.05);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.25);
  backdrop-filter: blur(8px);
  background: rgba(255, 255, 255, 0.03);
}
.panel-scroll .n-card + .n-card {
  margin-top: 14px;
}
</style>

<style scoped>
.app-shell {
  display: flex;
  flex-direction: column;
  height: 100%;
  height: 100dvh; /* 移动端安全 */
}
.app-body {
  flex: 1;
  min-height: 0;
  display: flex;
  overflow: hidden;
}
.app-main {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  /* 主区比侧栏亮一档，形成清晰明暗层次 */
  background: linear-gradient(180deg, #131c34 0%, #0e1628 100%);
}
</style>
