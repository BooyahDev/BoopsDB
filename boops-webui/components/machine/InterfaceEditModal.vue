<template>
  <v-dialog v-model="show" max-width="850">
    <v-card>
      <v-card-title class="pt-5 px-6">{{ selectedInterface?.name }} の名前・IPを編集</v-card-title>
      <v-card-text>
        <v-text-field v-model="name" label="NIC 名" :disabled="saving" />
        <v-btn color="primary" variant="tonal" class="mb-6" @click="saveName" :loading="saving" :disabled="name.trim() === selectedInterface?.name">名前を保存</v-btn>
        <p class="text-caption mb-3">IP 行の追加・削除は「IP を保存」で反映します。最後の行は削除できません。</p>
        <div v-for="(ip, index) in ips" :key="ip.rowId" class="ip-edit-row mb-3">
          <v-text-field v-model="ip.ip_address" :label="`IP アドレス ${index + 1}`" hide-details :disabled="saving" />
          <v-text-field v-model="ip.subnet_mask" label="サブネットマスク" hide-details :disabled="saving" />
          <v-checkbox v-model="ip.dns_register" label="iDNS 登録" hide-details density="compact" :disabled="saving" />
          <v-btn icon="mdi-delete-outline" variant="text" color="error" :aria-label="`IP 行 ${index + 1} を削除`" :disabled="ips.length === 1 || saving" @click="ips.splice(index, 1)" />
        </div>
        <v-btn prepend-icon="mdi-plus" variant="text" color="primary" :disabled="saving" @click="addIp">IP 行を追加</v-btn>
        <v-alert v-if="error" type="error" variant="tonal" class="mt-4">{{ error }}</v-alert>
      </v-card-text>
      <v-card-actions class="px-6 pb-5"><v-spacer /><v-btn :disabled="saving" @click="show = false">キャンセル</v-btn><v-btn color="primary" variant="flat" @click="saveIps" :loading="saving">IP を保存</v-btn></v-card-actions>
    </v-card>
  </v-dialog>
</template>
<script setup>
import { ref, computed, watch } from 'vue';
import { useInterfaceApi } from '@/composables/useInterfaceApi';
import { createIpEditRows, toIpPayload } from '@/utils/interfaceRows.js';
const props = defineProps({ modelValue: Boolean, selectedInterface: { type: Object, default: null }, machineId: { type: String, default: '' } });
const emit = defineEmits(['update:modelValue', 'saved']);
const { updateInterfaceIps, updateInterfaceName } = useInterfaceApi();
const show = computed({ get: () => props.modelValue, set: value => emit('update:modelValue', value) });
const ips = ref([]), name = ref(''), saving = ref(false), error = ref('');
let sequence = 0;
const nextId = () => `draft-${++sequence}`;
watch(() => [props.modelValue, props.selectedInterface], () => {
  if (!props.modelValue || !props.selectedInterface) return;
  ips.value = createIpEditRows(props.selectedInterface.ips, nextId);
  name.value = props.selectedInterface.name;
  error.value = '';
}, { immediate: true });
const addIp = () => { ips.value.push(...createIpEditRows([], nextId)); };
const saveName = async () => {
  if (!name.value.trim()) { error.value = 'NIC 名を入力してください。'; return; }
  saving.value = true; error.value = '';
  try { await updateInterfaceName(props.machineId, props.selectedInterface.name, name.value.trim()); show.value = false; emit('saved'); }
  catch (err) { error.value = err.message; }
  finally { saving.value = false; }
};
const saveIps = async () => {
  if (!ips.value.length || ips.value.some(ip => !ip.ip_address.trim())) { error.value = 'すべての行に IP アドレスを入力してください。'; return; }
  saving.value = true; error.value = '';
  try { await updateInterfaceIps(props.machineId, props.selectedInterface.name, toIpPayload(ips.value)); show.value = false; emit('saved'); }
  catch (err) { error.value = err.message; }
  finally { saving.value = false; }
};
</script>
<style scoped>
.ip-edit-row { display: grid; grid-template-columns: 1fr 1fr 130px 40px; align-items: center; gap: 10px; }
@media(max-width: 600px) { .ip-edit-row { grid-template-columns: 1fr 40px; border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity)); padding-bottom: 12px; } .ip-edit-row > :nth-child(1), .ip-edit-row > :nth-child(2) { grid-column: 1 / -1; } }
</style>
