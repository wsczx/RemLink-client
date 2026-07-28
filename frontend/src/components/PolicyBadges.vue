<template>
  <div class="badges">
    <n-tag v-for="b in badges" :key="b.label" :type="b.on ? 'success' : 'default'" :bordered="b.on" round>
      {{ b.label }}
    </n-tag>
    <n-tag v-if="policy?.policy.acl_count" type="info" round>
      ACL 规则 {{ policy.policy.acl_count }}
    </n-tag>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { NTag } from "naive-ui";
import type { PolicyInfo } from "../wailsjs/go/ClientService/models";

const props = defineProps<{ policy: PolicyInfo }>();

const badges = computed(() => {
  const p = props.policy.policy;
  return [
    { label: "IPv6 优先", on: p.prefer_ipv6 },
    { label: "压缩", on: p.compression },
    { label: "FakeDNS", on: p.fakedns },
    { label: "Split DNS", on: p.split_dns },
    { label: "允许局域网", on: p.allow_lan },
    { label: "断网保护", on: p.kill_switch },
  ];
});
</script>

<style scoped>
.badges {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
