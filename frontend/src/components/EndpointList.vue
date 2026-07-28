<template>
  <div class="ep-list">
    <div v-for="(ep, i) in endpoints" :key="ep" class="ep-item">
      <span class="ep-idx">{{ i + 1 }}</span>
      <span class="ep-host">{{ ep }}</span>
    </div>
    <div v-if="!endpoints.length" class="ep-empty">
      暂无接入点（连接后由服务端下发）
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ endpoints: string[] }>();
</script>

<style scoped>
.ep-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 10px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.03);
  margin-bottom: 6px;
}
.ep-idx {
  width: 20px;
  height: 20px;
  border-radius: 6px;
  background: rgba(14, 165, 233, 0.18);
  color: #38bdf8;
  font-size: 11px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
}
.ep-host {
  font-size: 12.5px;
  font-family: ui-monospace, "SFMono-Regular", Menlo, monospace;
}
.ep-empty {
  font-size: 12px;
  color: #9fb0d0;
  padding: 8px 0;
}
</style>
