// 等待 Wails 运行时与 ClientService 就绪。
// 注意：不能在脚本加载时直接捕获 window.go.services.ClientService，
// 否则 Wails 尚未注入完会抛 "Cannot access 'svc' before initialization"，
// 导致整个脚本中断（onConnect 未定义、init 不执行）。改为就绪后惰性获取。
function getSvc() {
  try {
    if (window.go && window.go.services && window.go.services.ClientService) {
      return window.go.services.ClientService;
    }
  } catch (e) { /* 尚未就绪 */ }
  return null;
}

function ready() {
  return new Promise((resolve) => {
    if (getSvc()) return resolve();
    const timer = setInterval(() => {
      if (getSvc()) { clearInterval(timer); resolve(); }
    }, 30);
  });
}

let durTimer = null;   // 时长计时器
let baseSecs = 0;     // 连接基准秒数（来自服务端 GetStatus）
let baseTime = 0;     // 本地基准时间戳
let eventsBound = false; // 防止重复打开窗口时重复订阅事件

// 无边框窗口拖拽：mousedown 时发 wails:drag（仅发起移动，双击不会最大化）。
// 不用 CSS -webkit-app-region（会变成系统标题栏 → 双击任意处触发 Windows
// 最大化且难以还原）。控件按钮（最小化/关闭）不触发拖拽。
function postWailsDrag() {
  try { if (window.chrome && chrome.webview && chrome.webview.postMessage) { chrome.webview.postMessage('wails:drag'); return; } } catch (e) {}
  try { if (window.webkit && webkit.messageHandlers && webkit.messageHandlers.wails) { webkit.messageHandlers.wails.postMessage('wails:drag'); return; } } catch (e) {}
}
function enableDrag() {
  [document.querySelector('.titlebar'), document.querySelector('.brand')].forEach(r => {
    if (!r) return;
    r.addEventListener('mousedown', (e) => {
      if (e.button !== 0) return;                 // 仅左键
      if (e.target.closest('.tb-controls')) return; // 点最小化/关闭按钮不拖拽
      postWailsDrag();
    });
  });
}

async function init() {
  await ready();
  const svc = getSvc();

  // 订阅引擎事件：status 为字符串（connecting/connected/disconnected/error），
  // error 事件携带 message；sso-login 由 Go 侧直接 SetURL 跳转，无需前端处理。
  // 用 eventsBound 去重，避免托盘反复「显示窗口」导致重复绑定。
  try {
    if (!eventsBound && window.wails && window.wails.Events && window.wails.Events.On) {
      window.wails.Events.On('event', onEngineEvent);
      eventsBound = true;
    }
  } catch (e) { /* 事件订阅失败不影响认证 */ }

  try {
    const brand = await svc.Brand();
    if (brand.name) document.getElementById('brandName').textContent = brand.name;
    if (brand.tagline) document.getElementById('brandTagline').textContent = brand.tagline;
    if (brand.version) document.getElementById('version').textContent = 'RemLink Client v' + brand.version;
  } catch (e) { /* 品牌信息缺失时使用默认值 */ }

  try {
    const servers = await svc.GetServers();
    renderSaved(servers || []);
    const last = await svc.LastServer();
    if (last) document.getElementById('server').value = last.replace(/^https?:\/\//, '');
  } catch (e) { /* 配置读取失败忽略 */ }

  // 初始状态：默认始终显示连接卡片（地址输入）。
  // 仅在 Go 端确认有活跃隧道数据时才切到详情页，避免残留状态导致未连接就显示「已安全连接」。
  showConnect();
  try {
    const st = await svc.GetStatus();
    if (st && st.status === 'connected' && st.tunnel) {
      showDetails();
      fillDetails(st.tunnel);
      if (st.duration_secs) { baseSecs = st.duration_secs; startTimer(); }
    } else if (st && st.error) {
      showErr(st.error);
    }
  } catch (e) { /* 忽略 */ }

  document.getElementById('connectBtn').addEventListener('click', onConnect);
  document.getElementById('disconnectBtn').addEventListener('click', onDisconnect);
  document.getElementById('portalBtn').addEventListener('click', onPortal);
  document.getElementById('minBtn').addEventListener('click', onMinimize);
  document.getElementById('hideBtn').addEventListener('click', onHide);

  // 无边框窗口：用 JS 发 wails:drag 实现拖拽（避免双击最大化）。
  enableDrag();
}

function renderSaved(servers) {
  // 已保存服务器折叠进输入框的 datalist（聚焦/输入时自动下拉），保持首页只有地址输入卡片。
  const dl = document.getElementById('savedServers');
  if (!dl) return;
  dl.innerHTML = '';
  (servers || []).forEach(s => {
    const opt = document.createElement('option');
    opt.value = (s.host || '').replace(/^https?:\/\//, '');
    dl.appendChild(opt);
  });
}

function showErr(m) {
  const err = document.getElementById('error');
  err.textContent = m; err.hidden = false;
}
function clearErr() {
  const err = document.getElementById('error');
  err.hidden = true; err.textContent = '';
}

async function onConnect() {
  const svc = getSvc();
  if (!svc) { showErr('客户端尚未就绪，请稍候重试'); return; }
  const input = document.getElementById('server');
  const btn = document.getElementById('connectBtn');
  const raw = input.value.trim();
  if (!raw) { showErr('请输入服务器地址'); return; }
  clearErr();
  btn.disabled = true;
  btn.querySelector('.btn-label').textContent = '连接中…';
  try {
    // Go 侧校验地址并持久化，随后异步发起连接：
    // 引擎发 sso-login 事件 → Go 把本页跳转到 WebAuth 登录页（非系统浏览器）。
    await svc.Connect(raw);
  } catch (e) {
    showErr((e && e.message) || String(e));
    btn.disabled = false;
    btn.querySelector('.btn-label').textContent = '连接';
  }
}

async function onDisconnect() {
  try { await getSvc().Disconnect(); } catch (e) { /* 忽略 */ }
}

async function onMinimize() {
  try { await getSvc().Minimize(); } catch (e) { /* 忽略 */ }
}

async function onHide() {
  try { await getSvc().HideWindow(); } catch (e) { /* 忽略 */ }
}

async function onPortal() {
  try { await getSvc().OpenPortal(); } catch (e) { showErr((e && e.message) || String(e)); }
}

// 引擎事件回调。
function onEngineEvent(ev) {
  const e = (ev && typeof ev.data !== 'undefined') ? ev.data : ev;
  if (!e) return;

  // sso-login：Go 侧已 SetURL 跳转到 WebAuth 登录页，本页即将被替换，无需处理。
  if (e.kind === 'sso-login') return;

  if (e.kind === 'status') {
    switch (e.status) {
      case 'connected': showDetails(); break;
      case 'connecting': break; // 等待
      case 'disconnected':
      case 'error': showConnect(); break;
    }
    return;
  }
  if (e.kind === 'tunnel' && e.tunnel) {
    fillDetails(e.tunnel);
  }
  if (e.kind === 'error' && e.message) {
    showErr(e.message);
  }
}

// 根据初始/拉取的状态渲染界面。
function renderStatus(st) {
  if (!st) return;
  if (st.status === 'connected') {
    showDetails();
    if (st.tunnel) fillDetails(st.tunnel);
    if (st.error) showErr(st.error);
    if (st.duration_secs) { baseSecs = st.duration_secs; startTimer(); }
  } else {
    showConnect();
    if (st.error) showErr(st.error);
  }
}

function showConnect() {
  stopTimer();
  document.getElementById('connectView').hidden = false;
  document.getElementById('detailsView').hidden = true;
  const btn = document.getElementById('connectBtn');
  btn.disabled = false;
  btn.querySelector('.btn-label').textContent = '连接';
  try { getSvc().SetSize(460, 560); } catch (e) {}
}

function showDetails() {
  stopTimer();
  document.getElementById('connectView').hidden = true;
  document.getElementById('detailsView').hidden = false;
  startTimer();
  try { getSvc().SetSize(460, 560); } catch (e) {}
}

function fillDetails(t) {
  if (!t) return;
  setText('ipv4', t.ipv4 || '—');
  setText('dns', (t.dns && t.dns.length) ? t.dns.join(', ') : '—');
  setText('mtu', t.mtu ? String(t.mtu) : '—');
  setText('server2', t.server || '—');
}

function setText(id, v) {
  const el = document.getElementById(id);
  if (el) el.textContent = v;
}

function startTimer() {
  stopTimer();
  baseTime = Date.now();
  const el = document.getElementById('duration');
  if (el) el.textContent = fmtDur(baseSecs);
  durTimer = setInterval(() => {
    const e = document.getElementById('duration');
    if (e) e.textContent = fmtDur(baseSecs + Math.floor((Date.now() - baseTime) / 1000));
  }, 1000);
}
function stopTimer() { if (durTimer) { clearInterval(durTimer); durTimer = null; } }
function fmtDur(s) {
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), x = s % 60;
  const p = n => String(n).padStart(2, '0');
  return p(h) + ':' + p(m) + ':' + p(x);
}

window.addEventListener('DOMContentLoaded', init);
