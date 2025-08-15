<template>
  <v-sheet class="mt-8">
    <h2 class="text-h5 mb-4">Add New Interface</h2>
    <v-form @submit.prevent="addInterface">
      <v-table>
        <tbody>
          <tr>
            <th width="20%">Name:</th>
            <td width="80%">
              <v-text-field 
                v-model="interfaceForm.name" 
                placeholder="eth0" 
                density="compact" 
                hide-details
                :rules="[required]"
              />
            </td>
          </tr>
          <tr>
            <th>IP Addresses:</th>
            <td>
              <div v-for="(ip, index) in interfaceForm.ips" :key="index" class="mb-2">
                <div class="d-flex align-center">
                  <v-text-field
                    v-model="ip.ip_address"
                    placeholder="192.168.1.100"
                    density="compact"
                    hide-details
                    class="mr-2"
                    :rules="[required]"
                  />
                  <v-text-field
                    v-model="ip.subnet_mask"
                    placeholder="255.255.255.0"
                    density="compact"
                    hide-details
                    class="mr-2"
                  />
                  <v-checkbox
                    v-model="ip.dns_register"
                    label="iDNS Regist"
                    hide-details
                    density="compact"
                    class="mr-2"
                  />
                  <v-btn
                    icon
                    color="error"
                    size="small"
                    @click="removeIp(index)"
                    v-if="interfaceForm.ips.length > 1"
                  >
                    <v-icon>mdi-delete</v-icon>
                  </v-btn>
                </div>
              </div>
              <v-btn
                color="primary"
                @click="addIp"
                prepend-icon="mdi-plus"
                size="small"
              >
                Add IP Address
              </v-btn>
            </td>
          </tr>
          <tr>
            <th>MAC Address:</th>
            <td>
              <v-text-field 
                v-model="interfaceForm.mac_address" 
                placeholder="00:1A:2B:3C:4D:5E" 
                density="compact" 
                hide-details
              />
            </td>
          </tr>
          <tr>
            <th>Gateway:</th>
            <td>
              <v-text-field 
                v-model="interfaceForm.gateway" 
                placeholder="192.168.1.1" 
                density="compact" 
                hide-details
              />
            </td>
          </tr>
          <tr>
            <th>DNS Servers:</th>
            <td>
              <v-text-field 
                v-model="interfaceForm.dns_servers" 
                placeholder="8.8.8.8,8.8.4.4" 
                density="compact" 
                hide-details
              />
            </td>
          </tr>
          <tr>
            <td colspan="2" class="text-right pt-4">
              <v-btn 
                color="primary" 
                type="submit" 
                :loading="isLoading"
              >
                Add Interface
              </v-btn>
              <v-alert v-if="error" type="error" density="compact" class="mt-2">
                {{ error }}
              </v-alert>
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-form>
  </v-sheet>
</template>

<script setup>
import { ref } from 'vue';
import { useInterfaceApi } from '@/composables/useInterfaceApi';

const props = defineProps({
  machineId: {
    type: String,
    required: true
  }
});

const emit = defineEmits(['added']);

const { createInterface } = useInterfaceApi();

const required = (value) => !!value || 'Required';

const interfaceForm = ref({
  name: '',
  mac_address: '',
  gateway: '',
  dns_servers: '',
  ips: [
    { ip_address: '', subnet_mask: '255.255.255.0', dns_register: false }
  ]
});

const isLoading = ref(false);
const error = ref('');

const addIp = () => {
  interfaceForm.value.ips.push({
    ip_address: '',
    subnet_mask: '255.255.255.0',
    dns_register: false
  });
};

const removeIp = (index) => {
  interfaceForm.value.ips.splice(index, 1);
};

const addInterface = async () => {
  // Validation
  if (!interfaceForm.value.name) {
    error.value = 'Interface name is required';
    return;
  }

  if (!interfaceForm.value.ips.some(ip => ip.ip_address)) {
    error.value = 'At least one IP address is required';
    return;
  }

  isLoading.value = true;
  error.value = '';

  try {
    // Prepare request data
    const requestData = {
      name: interfaceForm.value.name,
      mac_address: interfaceForm.value.mac_address || null,
      gateway: interfaceForm.value.gateway || null,
      dns_servers: interfaceForm.value.dns_servers 
        ? interfaceForm.value.dns_servers.split(',').map(s => s.trim()).filter(s => s)
        : null,
      ips: interfaceForm.value.ips
        .filter(ip => ip.ip_address)
        .map(ip => ({
          ip_address: ip.ip_address,
          subnet_mask: ip.subnet_mask || '255.255.255.0',
          dns_register: !!ip.dns_register
        }))
    };

    await createInterface(props.machineId, requestData);
    
    // Reset form
    interfaceForm.value = {
      name: '',
      mac_address: '',
      gateway: '',
      dns_servers: '',
      ips: [
        { ip_address: '', subnet_mask: '255.255.255.0', dns_register: false }
      ]
    };

    emit('added');
  } catch (err) {
    error.value = err.message;
  } finally {
    isLoading.value = false;
  }
};
</script>

<style scoped>
* { text-transform: none !important; }
</style>
