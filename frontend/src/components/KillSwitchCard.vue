<template>
  <n-card class="ks-card" title="断网保护 Kill Switch">
    <div class="ks-row">
      <span class="ks-state" :class="{ on }">{{ on ? "已启用（策略）" : "未启用" }}</span>
    </div>
    <p class="ks-desc">
      隧道中断窗口自动阻断明文出网，防止流量泄露。由管理员在接入策略中开启。
    </p>
  </n-card>
</template>

<script setup lang="ts">
defineProps<{ on: boolean }>();
</script>

<style scoped>
.ks-state {
  font-size: 14px;
  font-weight: 600;
  color: #9fb0d0;
}
.ks-state.on {
  color: #22c55e;
}
.ks-desc {
  font-size: 12px;
  color: #9fb0d0;
  line-height: 1.6;
  margin: 10px 0 0;
}
</style>
