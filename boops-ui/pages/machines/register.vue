<template>
  <div class="container">
    <h1>Register New Machine</h1>

    <!-- New Machine Form -->
    <form @submit.prevent="addMachine">
      <div>
        <label for="hostname">Hostname:</label>
        <input v-model="newMachine.hostname" type="text" id="hostname" required />
      </div>

      <div>
        <label for="cpu_info">CPU Info:</label>
        <input v-model="newMachine.cpu_info" type="text" id="cpu_info" />
      </div>

      <div>
        <label for="memory_size">Memory Size:</label>
        <input v-model="newMachine.memory_size" type="text" id="memory_size" />
      </div>

      <div>
        <label for="disk_info">Disk Info:</label>
        <input v-model="newMachine.disk_info" type="text" id="disk_info" />
      </div>

      <!-- Interface Form -->
      <div v-for="(interfaceData, index) in newMachine.interfaceNames" :key="index">
        <h2>Interface:
          <input
            type="text"
            :value="newMachine.interfaceNames[index]"
            @input="updateInterfaceName($event, index)"
            style="width: 150px; display: inline;"
          />
        </h2>
        <div>
          <label for="ip">IP Address:</label>
          <input v-model="getInterfaceData(index).ip_address" type="text" required />
        </div>
        <div>
          <label for="subnet">Subnet Mask:</label>
          <input v-model="getInterfaceData(index).subnet_mask" type="text" />
        </div>
        <div>
          <label for="gateway">Gateway:</label>
          <input v-model="getInterfaceData(index).gateway" type="text" />
        </div>
        <div>
          <label for="dns_servers">DNS Servers (comma-separated):</label>
          <input v-model="getInterfaceData(index).dns_servers" type="text" />
        </div>
      </div>

      <!-- Add another interface -->
      <button type="button" @click.prevent="addInterface">Add Interface</button>

      <button type="submit">Add Machine</button>
    </form>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';

const router = useRouter();
const newMachine = ref({
  hostname: '',
  cpu_info: '',
  memory_size: '',
  disk_info: '',
  interfaces: {},
  interfaceNames: ['eth0']
});

function initializeInterface() {
  return {
    ip_address: '',
    subnet_mask: '',
    gateway: '',
    dns_servers: ''
  };
}

// Initialize the first interface
newMachine.value.interfaces.eth0 = initializeInterface();

function getInterfaceData(index) {
  const name = newMachine.value.interfaceNames[index];
  if (!name || !newMachine.value.interfaces[name]) {
    // Create a new interface object if it doesn't exist yet
    newMachine.value.interfaces[name] = initializeInterface();
  }
  return newMachine.value.interfaces[name];
}

function updateInterfaceName(event, index) {
  const newName = event.target.value.trim();
  if (newName && newName !== newMachine.value.interfaceNames[index]) {
    // Get the current name at this index
    const oldName = newMachine.value.interfaceNames[index];

    // Update interface name in array
    newMachine.value.interfaceNames[index] = newName;

    // Move the interface data to the new name if it changed
    if (newName !== oldName) {
      Vue.set(newMachine.value.interfaces, newName, newMachine.value.interfaces[oldName]);
      delete newMachine.value.interfaces[oldName];
    }
  }
}

function addInterface() {
  const defaultName = `eth${Object.keys(newMachine.value.interfaces).length}`;
  // Add to both arrays simultaneously
  newMachine.value.interfaceNames.push(defaultName);
  Vue.set(newMachine.value.interfaces, defaultName, initializeInterface());
}

async function addMachine() {
  // Convert interfaces object to the format expected by API
  const formattedInterfaces = {};
  for (const name of newMachine.value.interfaceNames) {
    if (newMachine.value.interfaces[name]) {
      formattedInterfaces[name] = {
        ip_address: newMachine.value.interfaces[name].ip_address,
        subnet_mask: newMachine.value.interfaces[name].subnet_mask || '',
        gateway: newMachine.value.interfaces[name].gateway || '',
        dns_servers: newMachine.value.interfaces[name].dns_servers
          ? newMachine.value.interfaces[name].dns_servers.split(',').map(s => s.trim())
          : []
      };
    }
  }

  const response = await fetch('http://localhost:3001/api/machines', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      hostname: newMachine.value.hostname,
      cpu_info: newMachine.value.cpu_info || '',
      memory_size: newMachine.value.memory_size || '',
      disk_info: newMachine.value.disk_info || '',
      interfaces: formattedInterfaces
    })
  });

  if (response.ok) {
    // Redirect to machine list after successful registration
    router.push('/machines');
  } else {
    alert('Failed to add machine');
  }
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
form div {
  margin-bottom: 1rem;
}
label {
  font-weight: bold;
  display: block;
  margin-bottom: 0.5rem;
}
input[type="text"] {
  width: calc(100% - 24px);
  padding: 8px 12px;
  margin-top: 4px;
  border-radius: 4px;
  border: 1px solid #ccc;
}
button {
  display: inline-block;
  background-color: #007bff;
  color: white;
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  margin-right: 0.5rem;
}
button:hover {
  background-color: #0056b3;
}
</style>
