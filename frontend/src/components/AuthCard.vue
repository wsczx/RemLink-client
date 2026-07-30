<template>
  <n-card class="auth-card" :title="title">
    <n-alert v-if="form?.error" type="error" :title="form.error" />
    <div class="hint" v-if="form?.hint">{{ form.hint }}</div>

    <template v-if="form?.status === 'select_group'">
      <div class="group-tip">请选择要接入的用户组</div>
      <div class="group-list">
        <button
          v-for="g in (form.groups || [])"
          :key="g"
          class="group-item"
          :class="{ active: group === g }"
          @click="group = g"
        >
          <span class="gi-ico">◷</span>
          <span class="gi-name">{{ g }}</span>
          <span class="gi-check" v-if="group === g">✓</span>
        </button>
      </div>
    </template>
    <template v-else-if="form?.status === 'identify'">
      <div class="group-tip">请输入用户名以加载您可接入的用户组</div>
      <n-input v-model:value="identifyUser" placeholder="用户名" />
    </template>
    <template v-else-if="form?.status === 'credentials'">
      <n-input v-model:value="username" placeholder="用户名" />
      <n-input
        v-model:value="password"
        type="password"
        placeholder="密码"
        show-password-toggle
      />
    </template>
    <template v-else-if="form?.status === 'sms_phone'">
      <n-input v-model:value="phone" placeholder="手机号" />
    </template>
    <template v-else-if="['otp', 'sms', 'radius'].includes(form?.status || '')">
      <n-input v-model:value="code" :placeholder="codePlaceholder" />
      <div class="hint" v-if="form?.phone_masked">已发送验证码至 {{ form.phone_masked }}</div>
      <div class="hint" v-if="form?.challenge_msg">{{ form.challenge_msg }}</div>
    </template>
    <template v-else-if="form?.status === 'change_pwd'">
      <n-input
        v-model:value="newPwd"
        type="password"
        placeholder="新密码"
        show-password-toggle
      />
      <n-input
        v-model:value="newPwd2"
        type="password"
        placeholder="确认新密码"
        show-password-toggle
      />
    </template>

    <div class="auth-actions">
      <n-button type="primary" block :disabled="submitDisabled" :loading="submitting" @click="submit">
        下一步
      </n-button>
      <n-button block class="cancel-btn" @click="cancel">取消连接</n-button>
    </div>
  </n-card>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { NCard, NAlert, NInput, NButton } from "naive-ui";
import { useAuthStore } from "../stores/auth";

const auth = useAuthStore();
const form = computed(() => auth.form);

const group = ref("");
const identifyUser = ref("");
const username = ref("");
const password = ref("");
const phone = ref("");
const code = ref("");
const newPwd = ref("");
const newPwd2 = ref("");
const submitting = ref(false);

const title = computed(() => {
  switch (form.value?.status) {
    case "select_group":
      return "选择用户组";
    case "identify":
      return "身份认证";
    case "credentials":
      return "账号登录";
    case "sms_phone":
    case "sms":
      return "短信验证";
    case "otp":
      return "动态验证码";
    case "radius":
      return "挑战认证";
    case "change_pwd":
      return "修改密码";
    default:
      return "身份认证";
  }
});

// 选组步骤未选组时禁用「下一步」
const submitDisabled = computed(
  () =>
    (form.value?.status === "select_group" && !group.value) ||
    (form.value?.status === "identify" && !identifyUser.value),
);

const codePlaceholder = computed(() => {
  switch (form.value?.status) {
    case "otp":
      return "动态验证码";
    case "sms":
      return "短信验证码";
    case "radius":
      return "挑战码";
    default:
      return "验证码";
  }
});

// identify 阶段已输入用户名，进入凭据步骤时自动预填，避免用户重复输入。
watch(
  () => form.value?.status,
  (s) => {
    if (s === "credentials" && !username.value && form.value?.username) {
      username.value = form.value.username;
    }
  },
);

function buildAnswers(): Record<string, string> {
  const a: Record<string, string> = {};
  switch (form.value?.status) {
    case "select_group":
      a.group = group.value;
      break;
    case "identify":
      a.username = identifyUser.value;
      break;
    case "credentials":
      a.username = username.value || form.value?.username || "";
      a.password = password.value;
      break;
    case "sms_phone":
      a.phone = phone.value;
      break;
    case "otp":
    case "sms":
    case "radius":
      a.code = code.value;
      break;
    case "change_pwd":
      a.new_password = newPwd.value;
      a.new_password_confirm = newPwd2.value;
      break;
  }
  return a;
}

async function submit() {
  submitting.value = true;
  try {
    await auth.submit(buildAnswers());
    group.value = "";
    identifyUser.value = "";
    username.value = "";
    password.value = "";
    phone.value = "";
    code.value = "";
    newPwd.value = "";
    newPwd2.value = "";
  } finally {
    submitting.value = false;
  }
}

function cancel() {
  auth.cancel();
}
</script>

<style scoped>
.auth-card {
  width: min(400px, 92%);
}
.hint {
  font-size: 12px;
  color: #9fb0d0;
  margin: 12px 0;
  line-height: 1.6;
}
.auth-card :deep(.n-input),
.auth-card :deep(.n-select) {
  margin-bottom: 12px;
}

/* ── 组清单（列表式，像 webauth 那样） ── */
.group-tip {
  font-size: 12px;
  color: #8595ad;
  margin: 4px 0 10px;
}
.group-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}
.group-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 12px 14px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.03);
  color: #cbd5e1;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: all 0.15s ease;
}
.group-item:hover {
  border-color: rgba(14, 165, 233, 0.45);
  background: rgba(14, 165, 233, 0.08);
  color: #e2e8f0;
}
.group-item.active {
  border-color: #0ea5e9;
  background: rgba(14, 165, 233, 0.16);
  color: #e0f2fe;
  box-shadow: inset 0 0 0 1px rgba(14, 165, 233, 0.3);
}
.gi-ico {
  color: #0ea5e9;
  font-size: 14px;
  flex: none;
}
.gi-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.gi-check {
  color: #0ea5e9;
  font-size: 13px;
  flex: none;
}

/* ── 操作按钮：有间隔，取消也是实心按钮 ── */
.auth-actions {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 18px;
}
.cancel-btn {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  color: #a4b1c6;
}
.cancel-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  border-color: rgba(255, 255, 255, 0.2);
  color: #e2e8f0;
}
</style>
