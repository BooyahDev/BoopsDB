<template>
  <v-sheet class="mb-8">
    <div class="d-flex justify-space-between align-center mb-4">
      <h2 class="text-h5">
        Interface: {{ interfaceData.name }}
      </h2>
      <div>
        <v-btn 
          icon 
          variant="text" 
          size="small" 
          @click="$emit('edit', interfaceData)"
          class="mr-1"
        >
          <v-icon>mdi-pencil</v-icon>
        </v-btn>
        <v-btn 
          icon 
          variant="text" 
          size="small" 
          color="error" 
          @click="$emit('delete', interfaceData.id, interfaceData.name)"
        >
          <v-icon>mdi-delete</v-icon>
        </v-btn>
      </div>
    </div>

    <!-- IP Addresses Table -->
    <v-sheet class="mb-4">
      <h3 class="text-h6">IP Addresses</h3>
      <v-table class="elevation-1">
        <thead>
          <tr>
            <th width="30%">IP Address</th>
            <th width="30%">Subnet Mask</th>
            <th width="20%">iDNS Regist</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(ipData, ipIndex) in interfaceData.ips" :key="ipIndex">
            <td>
              {{ ipData.ip_address || 'N/A' }}
              <v-btn 
                icon 
                variant="text" 
                size="small" 
                @click="copyToClipboard(ipData.ip_address, 'ip-' + interfaceData.id + '-' + ipData.id)"
                class="ml-1"
              >
                <v-icon>
                  {{ copiedItems['ip-' + interfaceData.id + '-' + ipData.id] ? 'mdi-check' : 'mdi-content-copy' }}
                </v-icon>
              </v-btn>
            </td>
            <td>{{ ipData.subnet_mask || '255.255.255.0' }}</td>
            <td>{{ ipData.dns_register ? '✓ Yes' : '' }}</td>
          </tr>
          <tr v-if="interfaceData.ips.length === 0">
            <td colspan="2" class="text-center text-grey">No IP addresses configured</td>
          </tr>
        </tbody>
      </v-table>
    </v-sheet>

    <!-- Interface Details -->
    <v-table class="elevation-1">
      <tbody>
        <tr>
          <th width="20%">MAC Address:</th>
          <td width="80%">{{ interfaceData.mac_address || 'N/A' }}</td>
        </tr>
        <tr>
          <th>Gateway:</th>
          <td>
            <template v-if="isEditingGateway">
              <div class="d-flex align-center">
                <v-text-field 
                  v-model="gatewayEdit" 
                  density="compact" 
                  hide-details 
                  class="mr-2" 
                />
                <v-btn 
                  color="success" 
                  icon 
                  size="small" 
                  @click="saveGateway" 
                  :loading="isUpdatingGateway" 
                  class="mr-1"
                >
                  <v-icon>mdi-check</v-icon>
                </v-btn>
                <v-btn 
                  color="error" 
                  icon 
                  size="small" 
                  @click="cancelEditGateway"
                >
                  <v-icon>mdi-close</v-icon>
                </v-btn>
              </div>
            </template>
            <template v-else>
              {{ interfaceData.gateway || 'N/A' }}
              <v-btn 
                icon 
                variant="text" 
                size="small" 
                @click="enableEditGateway" 
                class="ml-1"
              >
                <v-icon>mdi-pencil</v-icon>
              </v-btn>
            </template>
          </td>
        </tr>
        <tr>
          <th>DNS Servers:</th>
          <td>
            <template v-if="isEditingDns">
              <div class="d-flex align-center">
                <v-text-field 
                  v-model="dnsEdit" 
                  density="compact" 
                  hide-details 
                  class="mr-2" 
                  placeholder="8.8.8.8,8.8.4.4"
                />
                <v-btn 
                  color="success" 
                  icon 
                  size="small" 
                  @click="saveDnsServers" 
                  :loading="isUpdatingDns" 
                  class="mr-1"
                >
                  <v-icon>mdi-check</v-icon>
                </v-btn>
                <v-btn 
                  color="error" 
                  icon 
                  size="small" 
                  @click="cancelEditDns"
                >
                  <v-icon>mdi-close</v-icon>
                </v-btn>
              </div>
            </template>
            <template v-else>
              {{ formatDnsServers(interfaceData.dns_servers) || 'N/A' }}
              <v-btn 
                icon 
                variant="text" 
                size="small" 
                @click="enableEditDns" 
                class="ml-1"
              >
                <v-icon>mdi-pencil</v-icon>
              </v-btn>
            </template>
          </td>
        </tr>
      </tbody>
    </v-table>
  </v-sheet>
</template>

<script setup>
import { ref } from 'vue';
import { useClipboard } from '@/composables/useClipboard';
import { useInterfaceApi } from '@/composables/useInterfaceApi';

const props = defineProps({
  interfaceData: {
    type: Object,
    required: true
  },
  machineId: {
    type: String,
    required: true
  }
});

const emit = defineEmits(['edit', 'delete', 'updated']);

const { copyToClipboard, copiedItems } = useClipboard();
const { updateInterfaceGateway, updateInterfaceDns } = useInterfaceApi();

// Gateway editing
const isEditingGateway = ref(false);
const gatewayEdit = ref('');
const isUpdatingGateway = ref(false);

// DNS editing
const isEditingDns = ref(false);
const dnsEdit = ref('');
const isUpdatingDns = ref(false);

// Gateway methods
const enableEditGateway = () => {
  gatewayEdit.value = props.interfaceData.gateway || '';
  isEditingGateway.value = true;
};

const cancelEditGateway = () => {
  isEditingGateway.value = false;
  gatewayEdit.value = '';
};

const saveGateway = async () => {
  isUpdatingGateway.value = true;
  try {
    await updateInterfaceGateway(props.machineId, props.interfaceData.name, gatewayEdit.value);
    emit('updated');
    isEditingGateway.value = false;
  } catch (error) {
    alert(`Failed to update gateway: ${error.message}`);
  } finally {
    isUpdatingGateway.value = false;
  }
};

// DNS methods
const enableEditDns = () => {
  dnsEdit.value = Array.isArray(props.interfaceData.dns_servers) 
    ? props.interfaceData.dns_servers.join(', ') 
    : (props.interfaceData.dns_servers || '');
  isEditingDns.value = true;
};

const cancelEditDns = () => {
  isEditingDns.value = false;
  dnsEdit.value = '';
};

const saveDnsServers = async () => {
  isUpdatingDns.value = true;
  try {
    const dnsArray = dnsEdit.value
      ? dnsEdit.value.split(',').map(item => item.trim()).filter(item => item)
      : [];
    
    await updateInterfaceDns(props.machineId, props.interfaceData.name, dnsArray);
    emit('updated');
    isEditingDns.value = false;
  } catch (error) {
    alert(`Failed to update DNS servers: ${error.message}`);
  } finally {
    isUpdatingDns.value = false;
  }
};

// Helper method
const formatDnsServers = (dns) => {
  if (!dns) return '';
  return Array.isArray(dns) ? dns.join(', ') : dns;
};
</script>

<style scoped>
* { text-transform: none !important; }
</style>
