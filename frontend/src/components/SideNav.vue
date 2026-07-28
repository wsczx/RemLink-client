<template>
  <nav class="sidebar">
    <!-- 品牌（原始官方 Logo） -->
    <div class="brand">
      <div class="brand-icon">
        <svg viewBox="0 0 64 64" width="34" height="34" aria-label="RemLink">
          <rect x="2" y="2" width="60" height="60" rx="14" fill="#0EA5E9"/>
          <path d="M14 40 L30 24 L38 32" fill="none" stroke="#F0F9FF" stroke-width="5" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M30 24 L50 24 L42 34" fill="none" stroke="#F0F9FF" stroke-width="5" stroke-linecap="round" stroke-linejoin="round"/>
          <circle cx="14" cy="40" r="5" fill="#BAE6FD"/>
          <circle cx="50" cy="24" r="5" fill="#BAE6FD"/>
          <circle cx="30" cy="24" r="4" fill="#F0F9FF"/>
        </svg>
      </div>
      <div class="brand-text">
        <div class="brand-name">RemLink</div>
        <div class="brand-sub">SECURE ACCESS</div>
      </div>
    </div>

    <!-- 导航：固定区域，不随内容拉伸 -->
    <div class="nav-wrap">
      <button
        v-for="item in items"
        :key="item.key"
        class="nav-item"
        :class="{ active: props.active === item.key }"
        @click="emit('update:active', item.key)"
      >
        <span class="nav-ico">{{ item.ico }}</span>
        <span class="nav-label">{{ item.label }}</span>
        <span v-if="item.key === 'notice' && notices.count" class="nav-badge">{{
          notices.count > 99 ? '99+' : notices.count
        }}</span>
      </button>
    </div>

    <!-- 底部版本（固定在侧栏底部） -->
    <div class="side-foot">
      <span class="ver">{{ version }}</span>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useNoticesStore } from "../stores/notices";
import { ClientService } from "../api/client";

const props = defineProps<{ active: string }>();
const emit = defineEmits<{ (e: "update:active", v: string): void }>();

const notices = useNoticesStore();
const items = [
  { key: "overview", ico: "◉", label: "概览" },
  { key: "policy", ico: "⚡", label: "策略" },
  { key: "notice", ico: "◈", label: "通知" },
  { key: "tools", ico: "⬡", label: "工具" },
];
const version = ref("—");

onMounted(() => {
  ClientService.Brand().then((b) => {
    if (b?.version) version.value = "v" + String(b.version);
  });
});
</script>

<style scoped>
.sidebar {
  width: 196px;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: linear-gradient(180deg, #161f38 0%, #101829 100%);
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  flex: none;
  overflow: hidden;
}

/* ── 品牌 ── */
.brand {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 20px 16px 18px;
  flex: none;
  user-select: none;
}
.brand-icon {
  width: 36px;
  height: 36px;
  flex: none;
}
.brand-name {
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 0.4px;
  color: #e2e8f0;
  line-height: 1.2;
}
.brand-sub {
  font-size: 9.5px;
  color: #64748b;
  letter-spacing: 1.8px;
  text-transform: uppercase;
  margin-top: 1px;
}

/* ── 导航区（顶部对齐，不随内容拉伸） ── */
.nav-wrap {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 8px 10px;
  min-height: 0;
  overflow-y: auto;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 12px;
  border: none;
  border-radius: 10px;
  background: transparent;
  color: #a4b1c6;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.18s ease;
  text-align: left;
  position: relative;
}
.nav-item:hover {
  background: rgba(255, 255, 255, 0.06);
  color: #eef3fb;
}
.nav-item.active {
  background: linear-gradient(135deg, rgba(14, 165, 233, 0.18), rgba(56, 189, 248, 0.08));
  color: #f0f9ff;
  font-weight: 600;
  box-shadow:
    inset 0 0 0 1px rgba(14, 165, 233, 0.25),
    0 4px 16px rgba(14, 165, 233, 0.1);
}
.nav-item.active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 18px;
  border-radius: 0 3px 3px 0;
  background: #0ea5e9;
  box-shadow: 0 0 8px rgba(14, 165, 233, 0.6);
}
.nav-ico {
  width: 20px;
  text-align: center;
  font-size: 14px;
  flex: none;
  opacity: 0.85;
}
.nav-item.active .nav-ico {
  opacity: 1;
}
.nav-label {
  flex: 1;
}
.nav-badge {
  font-size: 10px;
  min-width: 17px;
  height: 17px;
  padding: 0 5px;
  border-radius: 999px;
  background: linear-gradient(135deg, #0ea5e9, #0284c7);
  color: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  box-shadow: 0 2px 8px rgba(14, 165, 233, 0.4);
}

/* ── 底部版本 ── */
.side-foot {
  flex: none;
  padding: 12px 16px 14px;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
}
.ver {
  font-size: 10.5px;
  color: #475569;
  letter-spacing: 0.3px;
}
</style>
