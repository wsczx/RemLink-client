<template>
  <div class="panel-scroll">
    <div class="page-head">
      <h2>消息通知</h2>
      <n-button text type="error" :disabled="!notices.count" @click="notices.clear()"
        >清空</n-button
      >
    </div>
    <n-empty v-if="!notices.count" class="page-empty" description="暂无消息" />
    <div v-else class="notice-list">
      <NoticeItem v-for="(n, i) in notices.items" :key="i" :notice="n" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { NEmpty, NButton } from "naive-ui";
import { useNoticesStore } from "../stores/notices";
import NoticeItem from "../components/NoticeItem.vue";

const notices = useNoticesStore();
</script>

<style scoped>
.page-empty {
  margin-top: 80px;
}
.notice-list {
  display: flex;
  flex-direction: column;
}
</style>
