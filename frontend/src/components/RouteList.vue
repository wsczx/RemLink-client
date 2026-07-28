<template>
  <div class="kv">
    <div class="kv-block" v-if="includes.length">
      <div class="kv-head">路由包含</div>
      <div v-for="r in includes" :key="r" class="kv-item">{{ r }}</div>
    </div>
    <div class="kv-block" v-if="excludes.length">
      <div class="kv-head">路由排除</div>
      <div v-for="r in excludes" :key="r" class="kv-item">{{ r }}</div>
    </div>
    <div class="kv-empty" v-if="!includes.length && !excludes.length">无路由明细</div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ includes: string[]; excludes: string[] }>();
</script>

<style scoped>
.kv-block {
  margin-bottom: 12px;
}
.kv-head {
  font-size: 12px;
  color: #9fb0d0;
  margin-bottom: 6px;
}
.kv-item {
  font-size: 12.5px;
  font-family: ui-monospace, "SFMono-Regular", Menlo, monospace;
  padding: 4px 8px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.03);
  margin-bottom: 4px;
}
.kv-empty {
  font-size: 12px;
  color: #9fb0d0;
}
</style>
