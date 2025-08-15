<template>
  <v-container class="mt-6">
    <v-card v-if="machine" class="pa-6">
      <v-card-title class="d-flex justify-space-between align-center mb-6">
        <h1 class="text-h4 mt-5">{{ machine.hostname }}</h1>
      </v-card-title>

      <v-card-text>
        <!-- Machine Basic Information -->
        <MachineBasicInfo 
          :machine="machine" 
          @update:machine="handleMachineUpdate"
          @duplicate="handleDuplicate"
        />

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
      </v-card-text>
    </v-card>

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
        <v-card-title>Confirm Delete</v-card-title>
        <v-card-text>
          Are you sure you want to delete interface "{{ interfaceToDeleteName }}"?
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="error" @click="confirmDeleteInterface" :loading="isDeleting">
            Delete
          </v-btn>
          <v-btn color="secondary" @click="cancelDeleteInterface">
            Cancel
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
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { useMachineApi } from '@/composables/useMachineApi';
import { useInterfaceApi } from '@/composables/useInterfaceApi';

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
const loadMachine = async () => {
  try {
    machine.value = await getMachine(route.params.id);
  } catch (error) {
    alert('Failed to load machine details');
  }
};

// Handle machine updates
const handleMachineUpdate = (updatedMachine) => {
  machine.value = updatedMachine;
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
