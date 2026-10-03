<template>
  <v-container class="page-container">
    <div class="page-heading"><div><h1>マシン登録</h1><p class="page-subtitle">基本情報と NIC の設定を登録</p></div><v-btn to="/machines" variant="text" prepend-icon="mdi-arrow-left">一覧へ</v-btn></div>
    <v-form @submit.prevent="submitMachine">
      <v-card class="console-card mb-6"><v-card-title class="py-4">基本情報</v-card-title><v-divider /><v-card-text>
        <v-row>
          <v-col cols="12" md="6"><v-text-field v-model="machine.hostname" label="ホスト名（必須）" /></v-col>
          <v-col cols="12" md="6"><v-text-field v-model="machine.os_name" label="OS" /></v-col>
          <v-col cols="12" md="6"><v-text-field v-model="machine.cpu_info" label="CPU" /></v-col>
          <v-col cols="12" md="6"><v-select v-model="machine.cpu_arch" :items="['x86_64', 'arm64', 'i386', 'other']" label="CPU アーキテクチャ" /></v-col>
          <v-col cols="12" md="6"><v-text-field v-model="machine.memory_size" label="メモリー容量" /></v-col>
          <v-col cols="12" md="6"><v-text-field v-model="machine.disk_info" label="ディスク情報" /></v-col>
          <v-col cols="12"><v-text-field v-model="machine.purpose" label="用途" /></v-col>
          <v-col cols="12"><v-checkbox v-model="machine.is_virtual" label="仮想マシン" hide-details /></v-col>
          <v-col v-if="machine.is_virtual" cols="12"><v-text-field v-model="machine.parent_machine_id" label="親マシン UUID" /></v-col>
          <v-col cols="12"><v-textarea v-model="machine.memo" label="メモ" rows="3" /></v-col>
        </v-row>
      </v-card-text></v-card>
      <div class="page-heading mb-4"><h2 class="text-h5">ネットワークインターフェース</h2><v-btn prepend-icon="mdi-plus" variant="tonal" color="primary" @click="addInterface">NIC を追加</v-btn></div>
      <v-card v-for="(iface, index) in machine.interfaces" :key="iface.rowId" class="console-card mb-5">
        <v-card-title class="d-flex align-center ga-2 py-4"><v-icon>mdi-ethernet</v-icon><span>{{ iface.name || `NIC ${index + 1}` }}</span><v-spacer /><v-btn icon="mdi-delete-outline" variant="text" color="error" :aria-label="`NIC ${index + 1} を削除`" :disabled="machine.interfaces.length === 1" @click="machine.interfaces.splice(index, 1)" /></v-card-title><v-divider />
        <v-card-text>
          <v-row><v-col cols="12" md="6"><v-text-field v-model="iface.name" label="NIC 名（必須）" placeholder="eth0" /></v-col><v-col cols="12" md="6"><v-text-field v-model="iface.mac_address" label="MAC アドレス" /></v-col></v-row>
          <div v-for="(ip, ipIndex) in iface.ips" :key="ip.rowId" class="ip-row mb-3">
            <v-text-field v-model="ip.ip_address" :label="`IP アドレス ${ipIndex + 1}`" hide-details />
            <v-text-field v-model="ip.subnet_mask" label="サブネットマスク" hide-details />
            <v-checkbox v-model="ip.dns_register" label="iDNS 登録" hide-details />
            <v-btn icon="mdi-delete-outline" variant="text" color="error" :aria-label="`NIC ${index + 1} の IP 行 ${ipIndex + 1} を削除`" :disabled="iface.ips.length === 1" @click="iface.ips.splice(ipIndex, 1)" />
          </div>
          <v-btn variant="text" color="primary" prepend-icon="mdi-plus" @click="iface.ips.push(...createIpEditRows([], nextId))">IP 行を追加</v-btn>
          <v-checkbox :model-value="gatewayNic === iface.rowId" label="この NIC をデフォルトゲートウェイに使用" hide-details @update:model-value="value => gatewayNic = value ? iface.rowId : null" />
          <v-text-field v-if="gatewayNic === iface.rowId" v-model="iface.gateway" label="ゲートウェイ IPv4 アドレス" class="mt-3" />
          <v-text-field v-model="iface.dns_servers" label="DNS サーバー（カンマ区切り）" class="mt-4" />
        </v-card-text>
      </v-card>
      <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
      <div class="d-flex justify-end"><v-btn color="primary" type="submit" :loading="saving" size="large" prepend-icon="mdi-check">マシンを登録</v-btn></div>
    </v-form>
  </v-container>
</template>
<script setup>
import { ref, onMounted } from 'vue';
import { useApiBaseUrl } from '@/apiConfig';
import { createIpEditRows, toIpPayload } from '@/utils/interfaceRows.js';
const apiBaseUrl = useApiBaseUrl();
const route = useRoute(), router = useRouter();
let sequence = 0;
const nextId = () => `draft-${++sequence}`;
const blankInterface = () => ({ rowId: nextId(), name: '', mac_address: '', gateway: '', dns_servers: '', ips: createIpEditRows([], nextId) });
const machine = ref({ hostname: '', os_name: '', cpu_info: '', cpu_arch: 'x86_64', memory_size: '', disk_info: '', purpose: '', is_virtual: false, parent_machine_id: '', memo: '', interfaces: [blankInterface()] });
const gatewayNic = ref(null), saving = ref(false), error = ref('');
const addInterface = () => { machine.value.interfaces.push(blankInterface()); };
const submitMachine = async () => {
  error.value = '';
  if (!machine.value.hostname.trim()) { error.value = 'ホスト名を入力してください。'; return; }
  const names = new Set();
  for (const iface of machine.value.interfaces) {
    if (!iface.name.trim() || names.has(iface.name.trim())) { error.value = 'NIC 名を入力し、重複しない名前にしてください。'; return; }
    names.add(iface.name.trim());
    if (iface.ips.some(ip => !ip.ip_address.trim())) { error.value = `${iface.name} のすべての IP アドレスを入力してください。`; return; }
    if (gatewayNic.value === iface.rowId && (!iface.gateway.trim() || iface.gateway.trim() === '0.0.0.0')) { error.value = '使用するゲートウェイの IPv4 アドレスを入力してください。'; return; }
  }
  saving.value = true;
  try {
    const { interfaces, ...basic } = machine.value;
    const payload = { ...basic, hostname: basic.hostname.trim(), parent_machine_id: basic.is_virtual ? (basic.parent_machine_id || null) : null, interfaces: {} };
    for (const iface of interfaces) payload.interfaces[iface.name.trim()] = { ips: toIpPayload(iface.ips), mac_address: iface.mac_address || null, gateway: gatewayNic.value === iface.rowId ? iface.gateway.trim() : '', dns_servers: iface.dns_servers.split(',').map(s => s.trim()).filter(Boolean) };
    const response = await fetch(`${apiBaseUrl}/machines`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || 'マシンを登録できませんでした。');
    await router.push(`/machines/${data.id}`);
  } catch (err) { error.value = err.message; }
  finally { saving.value = false; }
};
onMounted(() => {
  if (!route.query.duplicate) return;
  try {
    const saved = JSON.parse(localStorage.getItem(route.query.duplicate) || 'null');
    if (!saved?.data || saved.expires <= Date.now()) return;
    const { id, interfaces, ...basic } = saved.data;
    machine.value = { ...machine.value, ...basic, interfaces: interfaces.map(iface => ({ ...blankInterface(), name: iface.name, mac_address: iface.mac_address || '', gateway: iface.gateway || '', dns_servers: Array.isArray(iface.dns_servers) ? iface.dns_servers.join(', ') : (iface.dns_servers || ''), ips: createIpEditRows(iface.ips.map(({ id, ...ip }) => ip), nextId) })) };
    const gatewayInterfaces = machine.value.interfaces.filter(iface => iface.gateway?.trim() && iface.gateway.trim() !== '0.0.0.0');
    gatewayNic.value = gatewayInterfaces.length === 1 ? gatewayInterfaces[0].rowId : null;
  } catch { error.value = '複製データを読み込めませんでした。'; }
});
</script>
<style scoped>
.ip-row { display: grid; grid-template-columns: 1fr 1fr 140px 40px; align-items: center; gap: 12px; }
@media(max-width: 600px) { .ip-row { grid-template-columns: 1fr 40px; border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); padding-bottom: 12px; } .ip-row > :nth-child(1), .ip-row > :nth-child(2) { grid-column: 1 / -1; } }
</style>
