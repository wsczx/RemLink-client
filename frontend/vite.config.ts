import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import { fileURLToPath, URL } from "node:url";

// 构建产物直接写入 Go 嵌入目录，go:embed 无需改动即可加载新前端。
// 旧手写 assets（index.html/app.js/bindings.js/styles.css）由 emptyOutDir 清空。
const outDir = fileURLToPath(new URL("../internal/ui/assets", import.meta.url));

export default defineConfig({
  plugins: [vue()],
  base: "/",
  build: {
    outDir,
    emptyOutDir: true,
    rollupOptions: {
      // Wails 运行时由 Go 中间件在 /wails/runtime.js 提供，保持为原生 ESM 外部导入。
      external: [/^\/wails\//],
    },
  },
  server: { port: 5173, strictPort: false },
});
