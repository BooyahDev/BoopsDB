<template>
  <div class="container" v-if="machine">
    <h1>{{ machine.hostname }} Details</h1>

      <!-- Machine Information -->
    <section>
      <h2>Main Information</h2>
      <table>
        <tr>
          <th>ID:</th>
          <td>
            {{ machine.id }}
            <button @click="copyToClipboard(machine.id, $event)" class="copy-btn">Copy</button>
          </td>
        </tr>
        <tr>
          <th>Hostname:</th>
          <td>
            {{ machine.hostname }}
            <button @click="copyToClipboard(machine.hostname, $event)" class="copy-btn">Copy</button>
          </td>
        </tr>
        <tr>
          <th>CPU Info:</th>
          <td>{{ machine.cpu_info || 'N/A' }}</td>
        </tr>
        <tr>
          <th>CPU Architecture:</th>
          <td>{{ machine.cpu_arch || 'N/A' }}</td>
        </tr>
        <tr>
          <th>Memory Size:</th>
          <td>{{ machine.memory_size || 'N/A' }}</td>
        </tr>
        <tr>
          <th>Disk Info:</th>
          <td>{{ machine.disk_info || 'N/A' }}</td>
        </tr>
        <tr>
          <th>Is Virtual Machine:</th>
          <td>{{ machine.is_virtual ? 'Yes' : 'No' }}</td>
        </tr>
        <tr>
          <th>Purpose:</th>
          <td>{{ machine.purpose || 'N/A' }}</td>
        </tr>
        <tr v-if="machine.is_virtual">
          <th>Parent Machine ID:</th>
          <td>
            {{ machine.parent_machine_id || 'N/A' }}
            <button @click="copyToClipboard(machine.parent_machine_id, $event)" class="copy-btn">Copy</button>
          </td>
        </tr>
        <tr v-if="machine.is_virtual && machine.parent_machine_id">
          <th>Parent Machine:</th>
          <td>
            <a :href="`/machines/${machine.parent_machine_id}`">{{ machine.parentHostname || machine.hostname || 'Unknown' }}</a>
          </td>
        </tr>
        <tr>
          <th>Memo:</th>
          <template v-if="isEditingMemo">
            <td>
              <textarea v-model="machine.memo" rows="3"></textarea>
              <button @click="updateMemo" :disabled="isUpdatingMemo">Save</button>
              <span v-if="isUpdatingMemo">Saving...</span>
              <button @click="cancelEditMemo">Cancel</button>
            </td>
          </template>
          <template v-else>
            <td>{{ machine.memo || 'N/A' }}
              <button @click="enableEditMemo">Edit</button>
            </td>
          </template>
        </tr>
      </table>
    </section>

      <!-- Interfaces -->
    <section v-for="(interfaceData, name) in machine.interfaces" :key="name">
      <h2 v-if="isEditingName[name]">
        Interface Name:
        <input v-model="interfaceData.name" />
        <button @click="saveInterfaceName(name)" :disabled="isLoadingName[name]">Save</button>
        <span v-if="isLoadingName[name]">Saving...</span>
        <button @click="cancelEditInterfaceName(name)">Cancel</button>
      </h2>
      <h2 v-else>
        Interface: {{ name }}
        <button @click="editInterfaceName(name)">Edit</button>
      </h2>
      <template v-if="isEditingIp[name]">
        <table>
          <tr>
            <th>IP Address:</th>
            <td>
              <input v-model="interfaceData.ip" />
              <button @click="saveNetworkSetting(name, 'ip')" :disabled="isLoadingIp[name]">Save</button>
              <span v-if="isLoadingIp[name]">Saving...</span>
              <button @click="cancelEditIp(name)">Cancel</button>
            </td>
          </tr>
          <tr>
            <th>MAC Address:</th>
            <td>{{ interfaceData.mac_address || 'N/A' }}</td>
          </tr>
          <template v-if="interfaceData">
            <tr>
              <th>Subnet Mask:</th>
              <template v-if="isEditingSubnet[name]">
                <td>
                  <input v-model="interfaceData.subnet" />
                  <button @click="saveNetworkSetting(name, 'subnet')" :disabled="isLoadingSubnet[name]">Save</button>
                  <span v-if="isLoadingSubnet[name]">Saving...</span>
                  <button @click="cancelEditNetwork(name, 'subnet')">Cancel</button>
                </td>
              </template>
              <template v-else>
                <td>{{ interfaceData.subnet || 'N/A' }}
                  <button @click="editSubnet(name)">Edit</button>
                </td>
              </template>
            </tr>
            <tr>
              <th>Gateway:</th>
              <template v-if="isEditingGateway[name]">
                <td>
                  <input v-model="interfaceData.gateway" />
                  <button @click="saveNetworkSetting(name, 'gateway')" :disabled="isLoadingGateway[name]">Save</button>
                  <span v-if="isLoadingGateway[name]">Saving...</span>
                  <button @click="cancelEditNetwork(name, 'gateway')">Cancel</button>
                </td>
              </template>
              <template v-else>
                <td>{{ interfaceData.gateway || 'N/A' }}
                  <button @click="editGateway(name)">Edit</button>
                </td>
              </template>
            </tr>
            <tr>
              <th>DNS Servers:</th>
              <template v-if="isEditingDns[name]">
                <td>
                  <input v-model="interfaceData.dns_servers" />
                  <button @click="saveNetworkSetting(name, 'dns')" :disabled="isLoadingDns[name]">Save</button>
                  <span v-if="isLoadingDns[name]">Saving...</span>
                  <button @click="cancelEditNetwork(name, 'dns')">Cancel</button>
                </td>
              </template>
              <template v-else>
                <td>{{ interfaceData.dns_servers.join(', ') }}
                  <button @click="editDns(name)">Edit</button>
                </td>
              </template>
            </tr>
          </template>
        </table>
      </template>
      <template v-else>
        <table>
          <tr>
            <th>IP Address:</th>
            <td>
              {{ interfaceData.ip || 'N/A' }}
              <button @click="copyToClipboard(interfaceData.ip, $event)" class="copy-btn">Copy</button>
              <button @click="editIp(name)">Edit</button>
            </td>
          </tr>
          <tr>
            <th>MAC Address:</th>
            <td>{{ interfaceData.mac_address || 'N/A' }}</td>
          </tr>
          <template v-if="interfaceData">
            <tr>
              <th>Subnet Mask:</th>
              <template v-if="isEditingSubnet[name]">
                <td>
                  <input v-model="interfaceData.subnet" />
                  <button @click="saveNetworkSetting(name, 'subnet')" :disabled="isLoadingSubnet[name]">Save</button>
                  <span v-if="isLoadingSubnet[name]">Saving...</span>
                  <button @click="cancelEditNetwork(name, 'subnet')">Cancel</button>
                </td>
              </template>
              <template v-else>
                <td>{{ interfaceData.subnet || 'N/A' }}
                  <button @click="editSubnet(name)">Edit</button>
                </td>
              </template>
            </tr>
            <tr>
              <th>Gateway:</th>
              <template v-if="isEditingGateway[name]">
                <td>
                  <input v-model="interfaceData.gateway" />
                  <button @click="saveNetworkSetting(name, 'gateway')" :disabled="isLoadingGateway[name]">Save</button>
                  <span v-if="isLoadingGateway[name]">Saving...</span>
                  <button @click="cancelEditNetwork(name, 'gateway')">Cancel</button>
                </td>
              </template>
              <template v-else>
                <td>{{ interfaceData.gateway || 'N/A' }}
                  <button @click="editGateway(name)">Edit</button>
                </td>
              </template>
            </tr>
            <tr>
              <th>DNS Servers:</th>
              <template v-if="isEditingDns[name]">
                <td>
                  <input v-model="interfaceData.dns_servers" />
                  <button @click="saveNetworkSetting(name, 'dns')" :disabled="isLoadingDns[name]">Save</button>
                  <span v-if="isLoadingDns[name]">Saving...</span>
                  <button @click="cancelEditNetwork(name, 'dns')">Cancel</button>
                </td>
              </template>
              <template v-else>
                <td>{{ interfaceData.dns_servers.join(', ') }}
                  <button @click="editDns(name)">Edit</button>
                </td>
              </template>
            </tr>
          </template>
        </table>
      </template>
    </section>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';

const route = useRoute();
const machine = ref(null);
const isEditingMemo = ref(false);
const isUpdatingMemo = ref(false); // New loading state for memo
const isEditingIp = ref({});
const isLoadingIp = ref({}); // Loading state for IP
const isEditingSubnet = ref({});
const isLoadingSubnet = ref({}); // Loading state for subnet
const isEditingGateway = ref({});
const isLoadingGateway = ref({}); // Loading state for gateway
const isEditingDns = ref({});
const isLoadingDns = ref({}); // Loading state for DNS
const isEditingName = ref({}); // New state for editing interface name
const isLoadingName = ref({}); // New loading state for interface name

function editIp(interfaceName) {
  isEditingIp.value[interfaceName] = true;
}

function cancelEditIp(interfaceName) {
  isEditingIp.value[interfaceName] = false;

  // Reload current values from server
  fetch(`http://localhost:3001/api/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value.interfaces[interfaceName] = data.interfaces[interfaceName];
    })
    .catch(err => console.error(err));
}

function editSubnet(interfaceName) {
  isEditingSubnet.value[interfaceName] = true;
}

function editGateway(interfaceName) {
  isEditingGateway.value[interfaceName] = true;
}

function editDns(interfaceName) {
  isEditingDns.value[interfaceName] = true;
}

function editInterfaceName(interfaceName) { // New function to enable editing interface name
  isEditingName.value[interfaceName] = true;
}

async function saveNetworkSetting(interfaceName, settingType) {
  const interfaceData = machine.value.interfaces[interfaceName];
  let url = '';
  let body = {};

  // Set loading state to true for the specific setting
  if (settingType === 'ip') {
    isLoadingIp.value[interfaceName] = true;
  } else if (settingType === 'subnet') {
    isLoadingSubnet.value[interfaceName] = true;
  } else if (settingType === 'gateway') {
    isLoadingGateway.value[interfaceName] = true;
  } else if (settingType === 'dns') {
    isLoadingDns.value[interfaceName] = true;
  }

  try {
    // Set up request based on setting type
    if (settingType === 'ip') {
      // Update IP address
      if (!interfaceData.ip) {
        alert('IP address cannot be empty');
        return;
      }

      url = `http://localhost:3001/api/interfaces/${machine.value.id}/${interfaceName}`;
      body = { ip_address: interfaceData.ip };

    } else if (settingType === 'subnet') {
      // Update subnet mask
      if (!interfaceData.subnet) {
        alert('Subnet mask cannot be empty');
        return;
      }

      url = `http://localhost:3001/api/interfaces/${machine.value.id}/${interfaceName}/update-subnet-mask`;
      body = { subnet_mask: interfaceData.subnet };

    } else if (settingType === 'gateway') {
      // Update gateway - Allow empty values
      url = `http://localhost:3001/api/interfaces/${machine.value.id}/${interfaceName}/update-gateway`;
      body = { gateway: interfaceData.gateway };

    } else if (settingType === 'dns') {
      // Update DNS servers - Allow empty values
      let dnsServersArray;
      if (typeof interfaceData.dns_servers === 'string' && interfaceData.dns_servers.trim()) {
        // Convert comma-separated string to array of trimmed values
        dnsServersArray = interfaceData.dns_servers.split(',').map(s => s.trim());
      } else if (Array.isArray(interfaceData.dns_servers)) {
        dnsServersArray = interfaceData.dns_servers;
      } else {
        // Allow empty array for blank DNS servers
        dnsServersArray = [];
      }

      url = `http://localhost:3001/api/interfaces/${machine.value.id}/${interfaceName}/update-dns`;
      body = { dns_servers: dnsServersArray };
    }

    const response = await fetch(url, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });

    if (!response.ok) {
      const errorData = await response.json();
      alert(`Failed to update ${settingType}: ${errorData.error || 'Unknown error'}`);
      return;
    }

    // Reset editing and loading states based on setting type
    if (settingType === 'ip') {
      isEditingIp.value[interfaceName] = false;
      isLoadingIp.value[interfaceName] = false;
    } else if (settingType === 'subnet') {
      isEditingSubnet.value[interfaceName] = false;
      isLoadingSubnet.value[interfaceName] = false;
    } else if (settingType === 'gateway') {
      isEditingGateway.value[interfaceName] = false;
      isLoadingGateway.value[interfaceName] = false;
    } else if (settingType === 'dns') {
      isEditingDns.value[interfaceName] = false;
      isLoadingDns.value[interfaceName] = false;
    }

  } catch (err) {
    console.error(err);
    alert(`An error occurred while updating the ${settingType}`);

    // Reset loading state on error
    if (settingType === 'ip') {
      isLoadingIp.value[interfaceName] = false;
    } else if (settingType === 'subnet') {
      isLoadingSubnet.value[interfaceName] = false;
    } else if (settingType === 'gateway') {
      isLoadingGateway.value[interfaceName] = false;
    } else if (settingType === 'dns') {
      isLoadingDns.value[interfaceName] = false;
    }
  }
}

async function saveInterfaceName(interfaceName) { // New function to save interface name
  const newName = machine.value.interfaces[interfaceName].name;

  // Set loading state to true
  isLoadingName.value[interfaceName] = true;

  try {
    const response = await fetch(`http://localhost:3001/api/interfaces/${machine.value.id}/${interfaceName}/update-name`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: newName })
    });

    if (!response.ok) {
      const errorData = await response.json();
      alert(`Failed to update interface name: ${errorData.error || 'Unknown error'}`);
      return;
    }

    // Reset editing and loading states
    isEditingName.value[interfaceName] = false;
    isLoadingName.value[interfaceName] = false;

  } catch (err) {
    console.error(err);
    alert('An error occurred while updating the interface name');

    // Reset loading state on error
    isLoadingName.value[interfaceName] = false;
  }
}

function cancelEditNetwork(interfaceName, settingType) {
  if (settingType === 'subnet') {
    isEditingSubnet.value[interfaceName] = false;
    isLoadingSubnet.value[interfaceName] = false; // Reset loading state
  } else if (settingType === 'gateway') {
    isEditingGateway.value[interfaceName] = false;
    isLoadingGateway.value[interfaceName] = false; // Reset loading state
  } else if (settingType === 'dns') {
    isEditingDns.value[interfaceName] = false;
    isLoadingDns.value[interfaceName] = false; // Reset loading state
  }

  // Reload current values from server
  fetch(`http://localhost:3001/api/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value.interfaces[interfaceName] = data.interfaces[interfaceName];
    })
    .catch(err => console.error(err));
}

function cancelEditInterfaceName(interfaceName) { // New function to cancel editing interface name
  isEditingName.value[interfaceName] = false;

  // Reload current values from server
  fetch(`http://localhost:3001/api/machines/${route.params.id}`)
    .then(response => response.json())
    .then(data => {
      machine.value.interfaces[interfaceName] = data.interfaces[interfaceName];
    })
    .catch(err => console.error(err));
}

onMounted(async () => {
  const response = await fetch(`http://localhost:3001/api/machines/${route.params.id}`);
  if (response.ok) {
    machine.value = await response.json();

    // If this is a virtual machine with a parent, fetch the parent machine details
    if (machine.value.is_virtual && machine.value.parent_machine_id) {
      const parentResponse = await fetch(`http://localhost:3001/api/machines/${machine.value.parent_machine_id}`);
      if (parentResponse.ok) {
        const parentMachine = await parentResponse.json();
        // Store the parent hostname to display in the link
        machine.value.parentHostname = parentMachine.hostname;
      } else {
        console.error('Failed to load parent machine details');
      }
    }
  } else {
    alert('Failed to load machine details');
  }
});

async function updateMemo() {
  isUpdatingMemo.value = true; // Set loading state

  const response = await fetch(`http://localhost:3001/api/machines/${machine.value.id}/update-memo`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ memo: machine.value.memo })
  });

  isUpdatingMemo.value = false; // Reset loading state

  if (response.ok) {
    isEditingMemo.value = false;
    alert('Memo updated successfully');
  } else {
    const errorData = await response.json();
    alert(`Failed to update memo: ${errorData.error || 'Unknown error'}`);
  }
}

function enableEditMemo() {
  isEditingMemo.value = true;
}

function cancelEditMemo() {
  isEditingMemo.value = false;
}

function copyToClipboard(id, event) {
  const button = event.target;
  navigator.clipboard.writeText(id).then(() => {
    button.innerText = 'Copied!';
    setTimeout(() => {
      button.innerText = 'Copy';
    }, 2000);
  }).catch(err => {
    console.error('Could not copy text: ', err);
  });
}
</script>

<style scoped>
.container {
  max-width: 800px;
  margin: auto;
  padding: 2rem;
}

h1, h2 {
  color: #333;
}

table {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 2rem;
}

th, td {
  border: 1px solid #ddd;
  padding: 0.5rem;
  text-align: left;
}

th {
  background-color: #f4f4f4;
  font-weight: bold;
}

.copy-btn {
  background-color: #007bff;
  color: white;
  border: none;
  padding: 0.25rem 0.5rem;
  cursor: pointer;
  margin-left: 0.5rem;
  border-radius: 4px;
}

.copy-btn:hover {
  background-color: #0069d9;
}
</style>
