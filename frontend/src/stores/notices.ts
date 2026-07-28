import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { ClientService, onEvent } from "../api/client";
import type { Notice, AppEvent } from "../wailsjs/go/ClientService/models";

export const useNoticesStore = defineStore("notices", () => {
  const items = ref<Notice[]>([]);
  const count = computed(() => items.value.length);

  function applyEvent(e: AppEvent) {
    if (e.kind === "notice" && e.notice) {
      items.value = [e.notice, ...items.value].slice(0, 50);
    }
  }

  async function load() {
    items.value = await ClientService.GetNotices();
  }

  async function clear() {
    await ClientService.ClearNotices();
    items.value = [];
  }

  onEvent(applyEvent);
  return { items, count, applyEvent, load, clear };
});
