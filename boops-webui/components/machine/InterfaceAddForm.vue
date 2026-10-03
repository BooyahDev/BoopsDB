<template>
  <v-card class="console-card mt-6">
    <v-card-title class="py-4"><v-icon class="mr-2">mdi-ethernet</v-icon>NIC を追加</v-card-title>
    <v-divider />
    <v-card-text>
      <v-form @submit.prevent="addInterface">
        <v-row><v-col cols="12" md="6"><v-text-field v-model="form.name" label="NIC 名" placeholder="eth2" /></v-col><v-col cols="12" md="6"><v-text-field v-model="form.mac_address" label="MAC アドレス" /></v-col></v-row>
        <div v-for="(ip, index) in form.ips" :key="ip.rowId" class="ip-add-row mb-3">
          <v-text-field v-model="ip.ip_address" :label="`IP アドレス ${index + 1}`" hide-details />
          <v-text-field v-model="ip.subnet_mask" label="サブネットマスク" hide-details />
          <v-checkbox v-model="ip.dns_register" label="iDNS 登録" hide-details />
          <v-btn icon="mdi-delete-outline" variant="text" color="error" :aria-label="`追加 IP 行 ${index + 1} を削除`" :disabled="form.ips.length === 1" @click="form.ips.splice(index, 1)" />
        </div>
        <v-btn prepend-icon="mdi-plus" variant="text" color="primary" class="mb-4" @click="form.ips.push(...createIpEditRows([], nextId))">IP 行を追加</v-btn>
        <v-checkbox v-model="useGateway" label="この NIC をデフォルトゲートウェイに使用" hide-details />
        <v-text-field v-if="useGateway" v-model="form.gateway" label="ゲートウェイ IPv4 アドレス" hint="保存時に他の NIC のゲートウェイを解除します。" persistent-hint class="mt-3" />
        <v-text-field v-model="form.dns_servers" label="DNS サーバー（カンマ区切り）" class="mt-4" />
        <v-alert v-if="error" type="error" variant="tonal" class="mb-4">{{ error }}</v-alert>
        <v-btn type="submit" color="primary" :loading="loading" prepend-icon="mdi-plus">NIC を追加</v-btn>
      </v-form>
    </v-card-text>
  </v-card>
</template>
<script setup>
import { ref } from 'vue';
import { useInterfaceApi } from '@/composables/useInterfaceApi';
import { createIpEditRows, toIpPayload } from '@/utils/interfaceRows.js';
const props = defineProps({ machineId: { type: String, required: true } });
const emit = defineEmits(['added']);
const { createInterface } = useInterfaceApi();
let sequence = 0;
const nextId = () => `new-${++sequence}`;
const blankForm = () => ({ name: '', mac_address: '', gateway: '', dns_servers: '', ips: createIpEditRows([], nextId) });
const form = ref(blankForm()), useGateway = ref(false), error = ref(''), loading = ref(false);
const addInterface = async () => {
  if (!form.value.name.trim() || form.value.ips.some(ip => !ip.ip_address.trim())) { error.value = 'NIC 名とすべての IP アドレスを入力してください。'; return; }
  if (useGateway.value && (!form.value.gateway.trim() || form.value.gateway.trim() === '0.0.0.0')) { error.value = '使用するゲートウェイの IPv4 アドレスを入力してください。'; return; }
  loading.value = true; error.value = '';
  try {
    await createInterface(props.machineId, { name: form.value.name.trim(), mac_address: form.value.mac_address || null, ips: toIpPayload(form.value.ips), gateway: useGateway.value ? form.value.gateway.trim() : '', dns_servers: form.value.dns_servers.split(',').map(s => s.trim()).filter(Boolean) });
    form.value = blankForm(); useGateway.value = false; emit('added');
  } catch (err) { error.value = err.message; }
  finally { loading.value = false; }
};
</script>
<style scoped>
.ip-add-row { display: grid; grid-template-columns: 1fr 1fr 140px 40px; align-items: center; gap: 12px; }
@media(max-width: 600px) { .ip-add-row { grid-template-columns: 1fr 40px; border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); padding-bottom: 12px; } .ip-add-row > :nth-child(1), .ip-add-row > :nth-child(2) { grid-column: 1 / -1; } }
</style>
