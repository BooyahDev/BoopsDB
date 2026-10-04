<template>
  <v-card class="console-card mb-5" :aria-label="`NIC ${interfaceData.name}`">
    <v-card-title class="d-flex align-center flex-wrap ga-2 py-4">
      <v-icon color="primary">mdi-ethernet</v-icon>
      <span class="text-h6 mono">{{ interfaceData.name }}</span>
      <v-chip v-if="hasGateway" color="primary" size="small">デフォルトゲートウェイに使用</v-chip>
      <v-spacer />
      <v-btn variant="text" :aria-label="`${interfaceData.name} の名前・IPを編集`" prepend-icon="mdi-pencil" @click="$emit('edit', interfaceData)">名前・IPを編集</v-btn>
      <v-btn icon="mdi-delete-outline" variant="text" color="error" :aria-label="`${interfaceData.name} を削除`" @click="$emit('delete', interfaceData.id, interfaceData.name)" />
    </v-card-title>
    <v-divider />
    <v-card-text>
      <h3 class="section-label">識別情報</h3>
      <v-row class="mb-3"><v-col cols="12" sm="4"><span class="text-medium-emphasis">NIC ID</span><div class="mono mt-1">{{ interfaceData.id }}</div></v-col><v-col cols="12" sm="8"><span class="text-medium-emphasis">MAC アドレス</span><div class="mono mt-1">{{ interfaceData.mac_address || '未設定' }}</div></v-col></v-row>
      <h3 class="section-label">IP アドレス</h3>
      <v-table density="comfortable" class="mb-5">
        <thead><tr><th>IP アドレス</th><th>サブネットマスク</th><th>iDNS 登録</th></tr></thead>
        <tbody><tr v-for="ip in interfaceData.ips" :key="ip.id"><td class="mono">{{ ip.ip_address }}<v-btn icon="mdi-content-copy" size="x-small" variant="text" :aria-label="`${ip.ip_address} をコピー`" @click="copyToClipboard(ip.ip_address, `ip-${ip.id}`)" /></td><td class="mono">{{ ip.subnet_mask }}</td><td>{{ ip.dns_register ? '登録する' : '登録しない' }}</td></tr></tbody>
      </v-table>
      <h3 class="section-label">ゲートウェイ・DNS</h3>
      <v-row>
        <v-col cols="12" md="6">
          <div class="text-medium-emphasis mb-2">デフォルトゲートウェイ</div>
          <template v-if="editingGateway">
            <v-text-field v-model="gatewayEdit" label="ゲートウェイ IPv4 アドレス" placeholder="192.168.1.1" :disabled="saving" />
            <p class="text-caption mb-3">保存すると、このマシンの他の NIC のゲートウェイを解除します。</p>
            <v-btn color="primary" @click="saveGateway(gatewayEdit)" :loading="saving">この NIC を使用</v-btn>
            <v-btn variant="text" class="ml-2" :disabled="saving" @click="editingGateway = false">キャンセル</v-btn>
          </template>
          <template v-else>
            <div class="mono mb-3">{{ hasGateway ? interfaceData.gateway : '使用しない' }}</div>
            <v-btn variant="tonal" color="primary" @click="beginGateway">{{ hasGateway ? 'ゲートウェイを編集' : 'この NIC を使用' }}</v-btn>
            <v-btn v-if="hasGateway" class="ml-2" variant="text" @click="saveGateway('')" :loading="saving">解除</v-btn>
          </template>
        </v-col>
        <v-col cols="12" md="6">
          <div class="text-medium-emphasis mb-2">DNS サーバー</div>
          <template v-if="editingDns">
            <v-text-field v-model="dnsEdit" label="DNS サーバー（カンマ区切り）" placeholder="8.8.8.8, 8.8.4.4" />
            <v-btn color="primary" @click="saveDns" :loading="saving">DNS を保存</v-btn><v-btn class="ml-2" variant="text" :disabled="saving" @click="editingDns = false">キャンセル</v-btn>
          </template>
          <template v-else><div class="mono mb-3">{{ formatDns(interfaceData.dns_servers) || '未設定' }}</div><v-btn variant="text" prepend-icon="mdi-pencil" @click="beginDns">DNS を編集</v-btn></template>
        </v-col>
      </v-row>
      <v-alert v-if="error" type="error" variant="tonal" class="mt-4">{{ error }}</v-alert>
    </v-card-text>
  </v-card>
</template>
<script setup>
import { ref, computed } from 'vue';
import { useClipboard } from '@/composables/useClipboard';
import { useInterfaceApi } from '@/composables/useInterfaceApi';
const props = defineProps({ interfaceData: { type: Object, required: true }, machineId: { type: String, required: true } });
const emit = defineEmits(['edit', 'delete', 'updated']);
const { copyToClipboard } = useClipboard();
const { updateInterfaceGateway, updateInterfaceDns } = useInterfaceApi();
const hasGateway = computed(() => !!props.interfaceData.gateway?.trim() && props.interfaceData.gateway.trim() !== '0.0.0.0');
const editingGateway = ref(false), editingDns = ref(false), gatewayEdit = ref(''), dnsEdit = ref(''), saving = ref(false), error = ref('');
const formatDns = dns => Array.isArray(dns) ? dns.join(', ') : (dns || '');
const beginGateway = () => { gatewayEdit.value = hasGateway.value ? props.interfaceData.gateway : ''; error.value = ''; editingGateway.value = true; };
const beginDns = () => { dnsEdit.value = formatDns(props.interfaceData.dns_servers); error.value = ''; editingDns.value = true; };
const saveGateway = async gateway => {
  const value = gateway.trim();
  if (editingGateway.value && (!value || value === '0.0.0.0')) { error.value = '使用するゲートウェイの IPv4 アドレスを入力してください。'; return; }
  saving.value = true; error.value = '';
  try { await updateInterfaceGateway(props.machineId, props.interfaceData.name, value); editingGateway.value = false; emit('updated'); }
  catch (err) { error.value = err.message; }
  finally { saving.value = false; }
};
const saveDns = async () => {
  saving.value = true; error.value = '';
  try { await updateInterfaceDns(props.machineId, props.interfaceData.name, dnsEdit.value.split(',').map(s => s.trim()).filter(Boolean)); editingDns.value = false; emit('updated'); }
  catch (err) { error.value = err.message; }
  finally { saving.value = false; }
};
</script>
<style scoped>.section-label { font-size: .9rem; font-weight: 600; margin-bottom: 12px; }</style>
