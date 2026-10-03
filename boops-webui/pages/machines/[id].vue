<template>
  <v-container class="page-container">
    <div class="page-heading"><div><v-btn :to="returnTo" variant="text" prepend-icon="mdi-arrow-left" size="small" class="mb-2">マシン一覧へ</v-btn><h1>{{ machine?.hostname || 'マシン詳細' }}</h1><p class="page-subtitle">基本情報とネットワーク設定</p></div></div>
    <v-alert v-if="loadError" type="error" variant="tonal" class="mb-5">{{ loadError }}</v-alert>
    <v-progress-linear v-if="loading" indeterminate color="primary" />
    <div v-if="machine">
        <!-- Machine Basic Information -->
        <MachineBasicInfo
          :machine="machine"
          @update:machine="handleMachineUpdate"
          @duplicate="handleDuplicate"
        />

<div class="page-heading mt-8 mb-4"><h2 class="text-h5">ネットワークインターフェース</h2><v-chip size="small">{{ machine.interfaces.length }} NIC</v-chip></div>
        <v-alert v-if="gatewayCount > 1" type="warning" variant="tonal" class="mb-5">複数の NIC にデフォルトゲートウェイが設定されています。利用する NIC の「この NIC を使用」で保存して、選択を 1 つに整理してください。</v-alert>
        <!-- Interface Cards -->
        <InterfaceCard
          v-for="interfaceData in machine.interfaces"
          :key="interfaceData.id"
          :interface-data="interfaceData"
          :machine-id="machine.id"
          @edit="handleEditInterface"
          @delete="handleDeleteInterface"
          @updated="loadMachine"
        />

        <!-- Add New Interface Form -->
        <InterfaceAddForm
          :machine-id="machine.id"
          @added="loadMachine"
        />

        <!-- Machine Actions -->
        <MachineActions :machine-id="machine.id" />
    </div>

    <!-- Interface Edit Modal -->
    <InterfaceEditModal
      v-model="showEditModal"
      :selected-interface="selectedInterface"
      :machine-id="machine?.id"
      @saved="loadMachine"
    />

    <!-- Interface Delete Confirmation -->
    <v-dialog v-model="showDeleteModal" max-width="500">
      <v-card>
        <v-card-title>NIC の削除</v-card-title>
        <v-card-text>
          NIC「{{ interfaceToDeleteName }}」を削除しますか？
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="error" @click="confirmDeleteInterface" :loading="isDeleting">
            削除
          </v-btn>
          <v-btn color="secondary" @click="cancelDeleteInterface">
            キャンセル
          </v-btn>
        </v-card-actions>
        <v-alert v-if="deleteError" type="error" density="compact" class="mx-4 mb-4">
          {{ deleteError }}
        </v-alert>
      </v-card>
    </v-dialog>
  </v-container>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue';
import { useRoute } from 'vue-router';
import { useMachineApi } from '@/composables/useMachineApi';
import { useInterfaceApi } from '@/composables/useInterfaceApi';
import { createMachineLoader } from '@/utils/machineLoader.js';

// Components
import MachineBasicInfo from '@/components/machine/MachineBasicInfo.vue';
import InterfaceCard from '@/components/machine/InterfaceCard.vue';
import InterfaceEditModal from '@/components/machine/InterfaceEditModal.vue';
import InterfaceAddForm from '@/components/machine/InterfaceAddForm.vue';
import MachineActions from '@/components/machine/MachineActions.vue';

const route = useRoute();
const { getMachine, duplicateMachine } = useMachineApi();
const { deleteInterface } = useInterfaceApi();

// Machine data
const machine = ref(null);
const loadError = ref(''), loading = ref(false);
const returnTo = computed(() => typeof route.query.returnTo === 'string' && /^\/machines(?:\?|$)/.test(route.query.returnTo) ? route.query.returnTo : '/machines');
const gatewayCount = computed(() => machine.value?.interfaces.filter(iface => iface.gateway?.trim() && iface.gateway.trim() !== '0.0.0.0').length || 0);

// Interface editing modal
const showEditModal = ref(false);
const selectedInterface = ref(null);

// Interface deletion
const showDeleteModal = ref(false);
const interfaceToDeleteId = ref('');
const interfaceToDeleteName = ref('');
const isDeleting = ref(false);
const deleteError = ref('');

// Load machine data
const loadMachine = createMachineLoader({
  getMachine,
  getMachineId: () => route.params.id,
  onLoading: value => { loading.value = value; },
  onError: message => { loadError.value = message; },
  onMachine: updated => {
    machine.value = updated;
    if (selectedInterface.value) {
      selectedInterface.value = updated.interfaces.find(iface => iface.id === selectedInterface.value.id) || null;
      if (!selectedInterface.value) showEditModal.value = false;
    }
  },
});

// Handle machine updates
const handleMachineUpdate = (updatedMachine) => {
  if (updatedMachine.id === route.params.id) loadMachine();
};

// Handle duplicate
const handleDuplicate = () => {
  duplicateMachine(machine.value);
};

// Handle interface editing
const handleEditInterface = (interfaceData) => {
  selectedInterface.value = interfaceData;
  showEditModal.value = true;
};

// Handle interface deletion
const handleDeleteInterface = (interfaceId, interfaceName) => {
  interfaceToDeleteId.value = interfaceId;
  interfaceToDeleteName.value = interfaceName;
  showDeleteModal.value = true;
  deleteError.value = '';
};

const confirmDeleteInterface = async () => {
  if (!machine.value) return;

  isDeleting.value = true;
  deleteError.value = '';

  try {
    await deleteInterface(machine.value.id, interfaceToDeleteName.value);
    showDeleteModal.value = false;
    interfaceToDeleteId.value = '';
    interfaceToDeleteName.value = '';
    await loadMachine();
  } catch (err) {
    deleteError.value = err.message;
  } finally {
    isDeleting.value = false;
  }
};

const cancelDeleteInterface = () => {
  showDeleteModal.value = false;
  interfaceToDeleteId.value = '';
  interfaceToDeleteName.value = '';
  deleteError.value = '';
};

watch(() => route.params.id, loadMachine);
// Initialize
onMounted(() => {
  loadMachine();
});
</script>

<style scoped>
* { text-transform: none !important; }

.container {
  max-width: 800px;
  margin: auto;
  padding: 2rem;
}
</style>
