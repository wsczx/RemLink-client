import { defineStore } from "pinia";
import { ref } from "vue";
import { ClientService, onEvent } from "../api/client";
import type { PolicyInfo, AppEvent } from "../wailsjs/go/ClientService/models";

export const usePolicyStore = defineStore("policy", () => {
  const policy = ref<PolicyInfo | null>(null);

  function applyEvent(e: AppEvent) {
    if (e.kind === "policy" && e.policy) policy.value = e.policy;
  }

  async function load() {
    policy.value = await ClientService.GetPolicy();
  }

  onEvent(applyEvent);
  return { policy, applyEvent, load };
});
