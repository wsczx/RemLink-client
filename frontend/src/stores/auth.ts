import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { ClientService, onEvent } from "../api/client";
import type { WebAuthForm, AppEvent } from "../wailsjs/go/ClientService/models";

export const useAuthStore = defineStore("auth", () => {
  const form = ref<WebAuthForm | null>(null);
  const visible = computed(() => !!form.value);

  function applyEvent(e: AppEvent) {
    if (e.kind === "webauth") form.value = e.webauth || null;
  }

  async function submit(answers: Record<string, string>) {
    await ClientService.SubmitAuth(answers);
  }

  function cancel() {
    form.value = null;
  }

  onEvent(applyEvent);
  return { form, visible, applyEvent, submit, cancel };
});
