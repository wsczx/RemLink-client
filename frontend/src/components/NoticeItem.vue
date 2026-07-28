<template>
  <div class="notice-item" :class="levelClass">
    <span class="ni-dot"></span>
    <div class="ni-body">
      <div class="ni-text">{{ notice.text }}</div>
      <div class="ni-time">{{ timeText }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { Notice } from "../wailsjs/go/ClientService/models";

const props = defineProps<{ notice: Notice }>();

const levelClass = computed(() => {
  switch (props.notice.level) {
    case "error":
      return "lv-error";
    case "warn":
      return "lv-warn";
    default:
      return "lv-info";
  }
});

const timeText = computed(() => {
  const d = new Date(props.notice.time * 1000);
  if (isNaN(d.getTime())) return "";
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(
    d.getHours(),
  )}:${p(d.getMinutes())}`;
});
</script>

<style scoped>
.notice-item {
  display: flex;
  gap: 10px;
  padding: 12px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.03);
  border-left: 3px solid transparent;
  margin-bottom: 8px;
}
.notice-item.lv-info {
  border-left-color: #0ea5e9;
}
.notice-item.lv-warn {
  border-left-color: #fbbf24;
}
.notice-item.lv-error {
  border-left-color: #f87171;
}
.ni-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-top: 6px;
  flex: none;
  background: #0ea5e9;
}
.lv-warn .ni-dot {
  background: #fbbf24;
}
.lv-error .ni-dot {
  background: #f87171;
}
.ni-text {
  font-size: 13px;
  line-height: 1.5;
}
.ni-time {
  font-size: 11px;
  color: #9fb0d0;
  margin-top: 4px;
}
</style>
