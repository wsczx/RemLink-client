<template>
  <div class="panel-scroll">
    <div class="page-head"><h2>工具</h2></div>
    <n-card title="智能选路" class="tool-card">
      <p class="tool-desc">已保存的接入入口，连接时自动测速择优（多活/异地部署）。</p>
      <EndpointList :endpoints="policy?.endpoints || []" />
    </n-card>

    <KillSwitchCard :on="!!policy?.policy.kill_switch" />

    <n-card title="网络诊断" class="tool-card">
      <p class="tool-desc">网关可达性 / DNS / MTU / 路由概况自检（需已建立隧道）。</p>
      <n-button :loading="diagLoading" :disabled="!conn.connected" @click="runDiag">
        运行网络诊断
      </n-button>
      <n-alert v-if="diagError" type="error" :title="diagError" />
      <pre v-if="diagText" class="diag-result">{{ diagText }}</pre>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { NCard, NButton, NAlert } from "naive-ui";
import { useConnectionStore } from "../stores/connection";
import { usePolicyStore } from "../stores/policy";
import { ClientService } from "../api/client";
import EndpointList from "../components/EndpointList.vue";
import KillSwitchCard from "../components/KillSwitchCard.vue";

const conn = useConnectionStore();
const policyStore = usePolicyStore();
const policy = computed(() => policyStore.policy);

const diagLoading = ref(false);
const diagError = ref("");
const diagText = ref("");

async function runDiag() {
  diagLoading.value = true;
  diagError.value = "";
  try {
    const r = (await ClientService.RunDiagnostics()) as Record<string, any>;
    diagText.value = JSON.stringify(r, null, 2);
  } catch (e) {
    diagError.value = String(e);
  } finally {
    diagLoading.value = false;
  }
}
</script>

<style scoped>
.tool-card {
  margin-bottom: 14px;
}
.tool-desc {
  font-size: 12px;
  color: #9fb0d0;
  line-height: 1.6;
  margin: 0 0 12px;
}
.diag-result {
  margin-top: 12px;
  padding: 12px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.3);
  font-size: 12px;
  font-family: ui-monospace, "SFMono-Regular", Menlo, monospace;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 280px;
  overflow: auto;
}
</style>
