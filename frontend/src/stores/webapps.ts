import { defineStore } from "pinia";
import { ref } from "vue";
import { ClientService } from "../api/client";

export interface WebApp {
  name: string;
  note: string;
  url: string;
}

export const useWebAppsStore = defineStore("webapps", () => {
  const apps = ref<WebApp[]>([]);
  const loading = ref(false);
  const loaded = ref(false);

  async function load() {
    if (loading.value || loaded.value) return;
    loading.value = true;
    try {
      const list = (await ClientService.WebVpnMyApps()) as WebApp[];
      apps.value = Array.isArray(list) ? list : [];
      loaded.value = true;
    } catch (e) {
      apps.value = [];
    } finally {
      loading.value = false;
    }
  }

  function open(app: WebApp) {
    if (!app.url) return;
    ClientService.OpenPortal(app.url).catch(() => {});
  }

  async function refresh() {
    loaded.value = false;
    apps.value = [];
    await load();
  }

  return { apps, loading, loaded, load, open, refresh };
});
