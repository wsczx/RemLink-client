<template>
  <div class="panel-scroll">
    <div class="page-head"><h2>访问策略</h2></div>
    <n-empty v-if="!policy" class="page-empty" description="暂无策略信息（连接后由服务端下发）" />
    <template v-else>
      <n-card title="策略概览">
        <div v-if="policy.policy_name" class="policy-name">
          {{ policy.policy_name
          }}<span v-if="policy.group"> · {{ policy.group }}</span>
        </div>
        <PolicyBadges :policy="policy" />
      </n-card>
      <n-card title="路由明细">
        <RouteList
          :includes="policy.routes_include || []"
          :excludes="policy.routes_exclude || []"
        />
        <div class="dns-block" v-if="policy.dns && policy.dns.length">
          <div class="kv-head">DNS</div>
          <div v-for="d in policy.dns" :key="d" class="kv-item">{{ d }}</div>
        </div>
      </n-card>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { NCard, NEmpty } from "naive-ui";
import { usePolicyStore } from "../stores/policy";
import PolicyBadges from "../components/PolicyBadges.vue";
import RouteList from "../components/RouteList.vue";

const policyStore = usePolicyStore();
const policy = computed(() => policyStore.policy);
</script>

<style scoped>
.page-empty {
  margin-top: 80px;
}
.policy-name {
  font-size: 15px;
  font-weight: 700;
  margin-bottom: 14px;
}
.dns-block {
  margin-top: 14px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  padding-top: 12px;
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
</style>
