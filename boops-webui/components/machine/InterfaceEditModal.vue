<template>
  <v-dialog v-model="show" max-width="800">
    <v-card>
      <v-card-title>
        <template v-if="isEditingInterfaceName">
          <v-text-field
            v-model="editingInterfaceName"
            label="Interface Name"
            density="compact"
            hide-details
            class="mb-2"
          />
        </template>
        <template v-else>
          Edit IP Addresses - {{ selectedInterface?.name || 'Unknown Interface' }}
          <v-btn
            icon
            variant="text"
            size="small"
            @click="enableEditInterfaceName"
            class="ml-2"
            :disabled="!selectedInterface"
          >
            <v-icon>mdi-pencil</v-icon>
          </v-btn>
        </template>
      </v-card-title>
      
      <v-card-text>
        <v-table>
          <thead>
            <tr>
              <th>IP Address</th>
              <th>Subnet Mask</th>
              <th>iDNS</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(ip, index) in editingIps" :key="index">
              <td>
                <v-text-field
                  v-model="ip.ip_address"
                  density="compact"
                  hide-details
                  placeholder="192.168.1.100"
                />
              </td>
              <td>
                <v-text-field
                  v-model="ip.subnet_mask"
                  density="compact"
                  hide-details
                  placeholder="255.255.255.0"
                />
              </td>
              <td>
                <v-checkbox
                  v-model="ip.dns_register"
                  label="iDNS Regist"
                  hide-details
                  density="compact"
                  class="ml-2" 
                />
              </td>
              <td>
                <v-btn
                  icon
                  color="error"
                  size="small"
                  @click="removeIpFromEdit(index)"
                >
                  <v-icon>mdi-delete</v-icon>
                </v-btn>
              </td>
            </tr>
            <tr>
              <td colspan="4" class="text-center pt-4">
                <v-btn
                  color="primary"
                  @click="addNewIpRow"
                  prepend-icon="mdi-plus"
                >
                  Add IP Row
                </v-btn>
              </td>
            </tr>
          </tbody>
        </v-table>
        
        <div v-if="isEditingInterfaceName" class="d-flex justify-end mt-4">
          <v-btn
            color="primary"
            @click="saveInterfaceName"
            :loading="isSavingInterfaceName"
            class="mr-2"
          >
            Save Name
          </v-btn>
          <v-btn
            color="secondary"
            @click="cancelEditInterfaceName"
          >
            Cancel
          </v-btn>
        </div>
        
        <v-alert v-if="error" type="error" density="compact" class="mt-4">
          {{ error }}
        </v-alert>
      </v-card-text>
      
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn
          color="primary"
          @click="saveInterfaceIps"
          :loading="isSavingInterfaceIps"
        >
          Save Changes
        </v-btn>
        <v-btn
          color="secondary"
          @click="cancel"
        >
          Cancel
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue';
import { useInterfaceApi } from '@/composables/useInterfaceApi';

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  selectedInterface: {
    type: Object,
    default: null
  },
  machineId: {
    type: String,
    required: true
  }
});

const emit = defineEmits(['update:modelValue', 'saved']);

const { updateInterfaceIps, updateInterfaceName } = useInterfaceApi();

const show = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
});

// State
const editingIps = ref([]);
const editingInterfaceName = ref('');
const isEditingInterfaceName = ref(false);
const isSavingInterfaceIps = ref(false);
const isSavingInterfaceName = ref(false);
const error = ref('');

// Watch for interface changes
watch(() => props.selectedInterface, (newInterface) => {
  if (newInterface) {
    editingIps.value = newInterface.ips?.length > 0
      ? newInterface.ips.map(ip => ({ ...ip }))
      : [{ ip_address: '', subnet_mask: '255.255.255.0', dns_register: false }];
    editingInterfaceName.value = newInterface.name || '';
    isEditingInterfaceName.value = false;
  }
}, { immediate: true });

// Interface name editing
const enableEditInterfaceName = () => {
  isEditingInterfaceName.value = true;
};

const cancelEditInterfaceName = () => {
  isEditingInterfaceName.value = false;
  editingInterfaceName.value = props.selectedInterface?.name || '';
};

const saveInterfaceName = async () => {
  if (!props.selectedInterface) return;

  const newName = editingInterfaceName.value.trim();
  if (!newName) {
    error.value = 'Interface name cannot be empty';
    return;
  }

  isSavingInterfaceName.value = true;
  error.value = '';

  try {
    await updateInterfaceName(props.machineId, props.selectedInterface.name, newName);
    isEditingInterfaceName.value = false;
    emit('saved');
  } catch (err) {
    error.value = err.message;
  } finally {
    isSavingInterfaceName.value = false;
  }
};

// IP editing
const addNewIpRow = () => {
  editingIps.value.push({
    ip_address: '',
    subnet_mask: '255.255.255.0',
    dns_register: false
  });
};

const removeIpFromEdit = (index) => {
  editingIps.value.splice(index, 1);
};

const isValidIp = (ip) => {
  const ipRegex = /^(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;
  return ipRegex.test(ip);
};

const saveInterfaceIps = async () => {
  if (!props.selectedInterface) return;

  isSavingInterfaceIps.value = true;
  error.value = '';

  try {
    // Validation
    const invalidIps = editingIps.value.filter(
      ip => ip.ip_address && !isValidIp(ip.ip_address)
    );
    
    if (invalidIps.length > 0) {
      throw new Error('Invalid IP address format');
    }

    // Filter empty IP addresses and prepare data
    const ipsToSave = editingIps.value
      .filter(ip => ip.ip_address.trim() !== '')
      .map(ip => ({
        ip_address: ip.ip_address,
        subnet_mask: ip.subnet_mask || '255.255.255.0',
        dns_register: !!ip.dns_register
      }));

    await updateInterfaceIps(props.machineId, props.selectedInterface.name, ipsToSave);
    show.value = false;
    emit('saved');
  } catch (err) {
    error.value = err.message;
  } finally {
    isSavingInterfaceIps.value = false;
  }
};

const cancel = () => {
  show.value = false;
  error.value = '';
};
</script>
