import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import { initEventBridge } from "./api/client";
import { useConnectionStore } from "./stores/connection";
import { usePolicyStore } from "./stores/policy";
import { useNoticesStore } from "./stores/notices";
import { useServersStore } from "./stores/servers";

const app = createApp(App);
app.use(createPinia());
app.mount("#app");

// 先建事件桥，再让各 store 注册 onEvent 监听（确保不丢事件）。
initEventBridge();
useConnectionStore().load();
usePolicyStore().load();
useNoticesStore().load();
useServersStore().load();
