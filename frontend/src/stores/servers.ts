import { defineStore } from "pinia";
import { ref } from "vue";
import { ClientService } from "../api/client";
import type { ServerEntry } from "../wailsjs/go/ClientService/models";

export const useServersStore = defineStore("servers", () => {
  const servers = ref<ServerEntry[]>([]);
  const last = ref("");

  async function load() {
    servers.value = await ClientService.GetServers();
    last.value = await ClientService.LastServer();
  }

  async function remove(raw: string) {
    await ClientService.RemoveServer(raw);
    await load();
  }

  return { servers, last, load, remove };
});
