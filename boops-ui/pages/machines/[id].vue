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
              <button @click="updateMemo">Save</button>
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
      <h2>Interface: {{ name }}</h2>
      <template v-if="isEditingIp[name]">
        <table>
          <tr>
            <th>IP Address:</th>
            <td>
              <input v-model="interfaceData.ip" />
              <button @click="saveNetworkSetting(name, 'ip')">Save</button>
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
              <template v-if="isEditingSubnet">
                <td>
                  <input v-model="interfaceData.subnet" />
                  <button @click="saveNetworkSetting(name, 'subnet')">Save</button>
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
              <template v-if="isEditingGateway">
                <td>
                  <input v-model="interfaceData.gateway" />
                  <button @click="saveNetworkSetting(name, 'gateway')">Save</button>
                  <button @click="cancelEditNetwork(name, 'gateway')">Cancel</button>
                </td>
              </template>
              <template v-else>
                <td>{{ interfaceData.gateway || 'N/A' }}
                  <button @click="editGateway(name)">Edit</button>
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
              <template v-if="isEditingSubnet">
                <td>
                  <input v-model="interfaceData.subnet" />
                  <button @click="saveNetworkSetting(name, 'subnet')">Save</button>
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
              <template v-if="isEditingGateway">
                <td>
                  <input v-model="interfaceData.gateway" />
                  <button @click="saveNetworkSetting(name, 'gateway')">Save</button>
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
              <th>DNS Severs:</th>
              <td>{{ interfaceData.dns_servers.join(', ') }}</td>
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
const isEditingIp = ref({});
const isEditingSubnet = ref(false);
const isEditingGateway = ref(false);

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
  isEditingSubnet.value = true;
}

function editGateway(interfaceName) {
  isEditingGateway.value = true;
}

async function saveNetworkSetting(interfaceName, settingType) {
  const interfaceData = machine.value.interfaces[interfaceName];
  let url = '';
  let body = {};

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
    url = `http://localhost:3001/api/interfaces/${machine.value.id}/${interfaceName}/update-subnet-mask`;
    body = { subnet_mask: interfaceData.subnet };

  } else if (settingType === 'gateway') {
    // Update gateway
    url = `http://localhost:3001/api/interfaces/${machine.value.id}/${interfaceName}/update-gateway`;
    body = { gateway: interfaceData.gateway };
  }

  try {
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

    // Reset editing state based on setting type
    if (settingType === 'ip') {
      isEditingIp.value[interfaceName] = false;
      isEditingSubnet.value = false;
      isEditingGateway.value = false;
    } else if (settingType === 'subnet') {
      isEditingSubnet.value = false;
    } else if (settingType === 'gateway') {
      isEditingGateway.value = false;
    }

  } catch (err) {
    console.error(err);
    alert(`An error occurred while updating the ${settingType}`);
  }
}

function cancelEditNetwork(interfaceName, settingType) {
  if (settingType === 'subnet') {
    isEditingSubnet.value = false;
  } else {
    isEditingGateway.value = false;
  }

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
  const response = await fetch(`http://localhost:3001/api/machines/${machine.value.id}/update-memo`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ memo: machine.value.memo })
  });

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
