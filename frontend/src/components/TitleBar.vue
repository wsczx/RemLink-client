<template>
  <header class="titlebar" @mousedown="onDrag">
    <div class="tb-controls" @mousedown.stop>
      <button class="tb-btn" title="最小化" @click="min">–</button>
      <button class="tb-btn tb-close" title="关闭" @click="hide">×</button>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ClientService } from "../api/client";

// 无边框窗口拖拽：发 wails:drag（避免双击任意处触发最大化）。
function onDrag() {
  try {
    const c = (window as any).chrome;
    if (c && c.webview && c.webview.postMessage) {
      c.webview.postMessage("wails:drag");
      return;
    }
  } catch {
    /* ignore */
  }
  try {
    const w = (window as any).webkit;
    if (w && w.messageHandlers && w.messageHandlers.wails) {
      w.messageHandlers.wails.postMessage("wails:drag");
      return;
    }
  } catch {
    /* ignore */
  }
}
function min() {
  void ClientService.Minimize();
}
function hide() {
  void ClientService.HideWindow();
}
</script>

<style scoped>
.titlebar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 10;
  flex: none;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 0 14px;
  /* 透明覆盖层，不占文档流，无背景无边框，完全融入主窗口 */
  background: transparent;
  border: none;
  user-select: none;
}
.tb-controls {
  display: flex;
  gap: 6px;
  flex: none;
}
.tb-btn {
  width: 28px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.04);
  color: #8595ad;
  font-size: 15px;
  line-height: 1;
  cursor: pointer;
  border-radius: 8px;
  transition: 0.15s;
  backdrop-filter: blur(6px);
}
.tb-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #eaf1ff;
  border-color: rgba(255, 255, 255, 0.16);
}
.tb-btn.tb-close:hover {
  background: rgba(248, 113, 113, 0.18);
  color: #f87171;
  border-color: rgba(248, 113, 113, 0.35);
}
</style>
