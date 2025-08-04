<template>
  <v-container class="mt-8">
    <v-card class="pa-6">
      <v-card-title class="mb-6">
        <h1 class="text-h4">マシン新規登録</h1>
      </v-card-title>

      <v-card-text>
        <v-form @submit.prevent="submitMachine">
          <!-- 基本情報セクション -->
          <v-sheet class="mb-8">
            <h2 class="text-h5 mb-4">Basic Information</h2>
            <v-table class="elevation-1">
              <tbody>
                <tr>
                  <th width="20%">Hostname:</th>
                  <td width="80%">
                    <v-text-field
                      v-model="machine.hostname"
                      density="compact"
                      hide-details
                      :rules="[required]"
                      required
                    />
                  </td>
                </tr>
                <tr>
                  <th>OS:</th>
                  <td>
                    <v-text-field
                      v-model="machine.os_name"
                      density="compact"
                      hide-details
                    />
                  </td>
                </tr>
                <tr>
                  <th>CPU Info:</th>
                  <td>
                    <v-text-field
                      v-model="machine.cpu_info"
                      density="compact"
                      hide-details
                    />
                  </td>
                </tr>
                <tr>
                  <th>CPU Architecture:</th>
                  <td>
                    <v-select
                      v-model="machine.cpu_arch"
                      :items="['x86_64', 'arm64', 'i386', 'other']"
                      density="compact"
                      hide-details
                    />
                  </td>
                </tr>
                <tr>
                  <th>Memory Size:</th>
                  <td>
                    <v-text-field
                      v-model="machine.memory_size"
                      density="compact"
                      hide-details
                    />
                  </td>
                </tr>
                <tr>
                  <th>Disk Info:</th>
                  <td>
                    <v-text-field
                      v-model="machine.disk_info"
                      density="compact"
                      hide-details
                    />
                  </td>
                </tr>
                <tr>
                  <th>Is Virtual Machine:</th>
                  <td>
                    <v-checkbox
                      v-model="machine.is_virtual"
                      label="Is Virtual Machine"
                      hide-details
                      density="compact"
                    />
                  </td>
                </tr>
                <tr v-if="machine.is_virtual">
                  <th>Parent Machine ID:</th>
                  <td>
                    <v-text-field
                      v-model="machine.parent_machine_id"
                      density="compact"
                      hide-details
                    />
                  </td>
                </tr>
                <tr>
                  <th>Purpose:</th>
                  <td>
                    <v-text-field
                      v-model="machine.purpose"
                      density="compact"
                      hide-details
                    />
                  </td>
                </tr>
                <tr>
                  <th>Memo:</th>
                  <td>
                    <v-textarea
                      v-model="machine.memo"
                      rows="3"
                      density="compact"
                      hide-details
                    />
                  </td>
                </tr>
              </tbody>
            </v-table>
          </v-sheet>

          <!-- インターフェースセクション -->
          <v-sheet v-for="(interfaceData, index) in machine.interfaces" :key="index" class="mb-8">
            <div class="d-flex justify-space-between align-center mb-4">
              <h2 class="text-h5">
                Interface: {{ interfaceData.name || `Interface ${index + 1}` }}
              </h2>
              <div>
                <v-btn
                  icon
                  variant="text"
                  size="small"
                  color="error"
                  @click="removeInterface(index)"
                >
                  <v-icon>mdi-delete</v-icon>
                </v-btn>
              </div>
            </div>

            <v-table class="elevation-1">
              <tbody>
                <tr>
                  <th width="20%">Name:</th>
                  <td width="80%">
                    <v-text-field
                      v-model="interfaceData.name"
                      placeholder="eth0"
                      density="compact"
                      hide-details
                      :rules="[required]"
                      required
                    />
                  </td>
                </tr>
                <tr>
                  <th>IP Addresses:</th>
                  <td>
                    <div v-for="(ip, ipIndex) in interfaceData.ips" :key="ipIndex" class="mb-2">
                      <div class="d-flex align-center">
                        <v-text-field
                          v-model="ip.ip_address"
                          placeholder="192.168.1.100"
                          density="compact"
                          hide-details
                          class="mr-2"
                          :rules="[required]"
                          required
                        />
                        <v-text-field
                          v-model="ip.subnet_mask"
                          placeholder="255.255.255.0"
                          density="compact"
                          hide-details
                          class="mr-2"
                        />
                        <v-btn
                          icon
                          color="error"
                          size="small"
                          @click="removeIp(index, ipIndex)"
                          v-if="interfaceData.ips.length > 1"
                        >
                          <v-icon>mdi-delete</v-icon>
                        </v-btn>
                        <v-checkbox
                          v-model="ip.dns_servers"
                          label="iDNS Regist"
                          hide-details
                          density="compact"
                          class="ml-2"
                        />
                      </div>
                    </div>
                    <v-btn
                      color="primary"
                      @click="addIp(index)"
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
                      v-model="interfaceData.mac_address"
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
                      v-model="interfaceData.gateway"
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
                      v-model="interfaceData.dns_servers"
                      placeholder="8.8.8.8,8.8.4.4"
                      density="compact"
                      hide-details
                    />
                  </td>
                </tr>
              </tbody>
            </v-table>
          </v-sheet>

          <!-- 新しいインターフェース追加 -->
          <v-sheet class="mt-8">
            <div class="d-flex justify-space-between align-center mb-4">
              <h2 class="text-h5">Add New Interface</h2>
              <v-btn
                color="primary"
                @click="addInterface"
                prepend-icon="mdi-plus"
              >
                Add Interface
              </v-btn>
            </div>
          </v-sheet>

          <!-- 送信ボタン -->
          <div class="mt-8 text-right">
            <v-btn
              color="primary"
              type="submit"
              :loading="isSubmitting"
              size="large"
            >
              Register Machine
            </v-btn>
            <v-alert
              v-if="submitError"
              type="error"
              density="compact"
              class="mt-4"
            >
              {{ submitError }}
            </v-alert>
          </div>
        </v-form>
      </v-card-text>
    </v-card>
  </v-container>
</template>

<script setup>
const required = (value) => !!value || 'Required';
import { apiBaseUrl } from '@/apiConfig';

const machine = ref({
  hostname: '',
  model_info: '',
  usage_desc: '',
  cpu_info: '',
  cpu_arch: 'x86_64',
  memory_size: '',
  disk_info: '',
  is_virtual: false,
  parent_machine_id: '',
  memo: '',
  interfaces: [
    {
      name: '',
      mac_address: '',
      gateway: '',
      dns_servers: '',
      ips: [
        { ip_address: '', subnet_mask: '255.255.255.0' }
      ]
    }
  ]
});

const isSubmitting = ref(false);
const submitError = ref('');

// インターフェースを追加
const addInterface = () => {
  machine.value.interfaces.push({
    name: '',
    mac_address: '',
    gateway: '',
    dns_servers: '',
    ips: [
      { ip_address: '', subnet_mask: '255.255.255.0' }
    ]
  });
};

// インターフェースを削除
const removeInterface = (index) => {
  if (machine.value.interfaces.length > 1) {
    machine.value.interfaces.splice(index, 1);
  } else {
    // 最後のインターフェースは削除できないようにする
    submitError.value = 'At least one interface is required';
    setTimeout(() => { submitError.value = ''; }, 3000);
  }
};

// IPアドレスを追加
const addIp = (interfaceIndex) => {
  machine.value.interfaces[interfaceIndex].ips.push({
    ip_address: '',
    subnet_mask: '255.255.255.0'
  });
};

// IPアドレスを削除
const removeIp = (interfaceIndex, ipIndex) => {
  if (machine.value.interfaces[interfaceIndex].ips.length > 1) {
    machine.value.interfaces[interfaceIndex].ips.splice(ipIndex, 1);
  } else {
    // 最後のIPアドレスは削除できないようにする
    submitError.value = 'At least one IP address is required per interface';
    setTimeout(() => { submitError.value = ''; }, 3000);
  }
};

// マシンを登録
const submitMachine = async () => {
  // バリデーション
  if (!machine.value.hostname) {
    submitError.value = 'Hostname is required';
    return;
  }

  for (const intf of machine.value.interfaces) {
    if (!intf.name) {
      submitError.value = `Interface name is required for all interfaces`;
      return;
    }

    if (!intf.ips.some(ip => ip.ip_address)) {
      submitError.value = `At least one IP address is required for interface ${intf.name || 'unnamed'}`;
      return;
    }
  }

  isSubmitting.value = true;
  submitError.value = '';

  try {
    // リクエストデータを整形
    const requestData = {
      hostname: machine.value.hostname,
      model_info: machine.value.model_info,
      usage_desc: machine.value.usage_desc,
      cpu_info: machine.value.cpu_info,
      cpu_arch: machine.value.cpu_arch,
      memory_size: machine.value.memory_size,
      disk_info: machine.value.disk_info,
      is_virtual: machine.value.is_virtual,
      parent_machine_id: machine.value.parent_machine_id || null,
      memo: machine.value.memo,
      interfaces: {}
    };

    // インターフェースデータを整形
    for (const intf of machine.value.interfaces) {
      requestData.interfaces[intf.name] = {
        ips: intf.ips
          .filter(ip => ip.ip_address)
          .map(ip => ({
            ip_address: ip.ip_address,
            subnet_mask: ip.subnet_mask || '255.255.255.0',
            dns_register: !!ip.dns_register
          })),
        gateway: intf.gateway || null,
        dns_servers: intf.dns_servers
          ? intf.dns_servers.split(',').map(s => s.trim()).filter(s => s)
          : null,
        mac_address: intf.mac_address || null
      };
    }

    const response = await fetch(`${apiBaseUrl}/machines`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(requestData)
    });

    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(errorData.error || 'Failed to register machine');
    }

    // 登録成功後の処理
    const result = await response.json();
    alert('Machine registered successfully!');
    // ここで適切なページにリダイレクトする
    // router.push(`/machines/${result.id}`);
  } catch (err) {
    console.error('Failed to register machine:', err);
    submitError.value = err.message;
  } finally {
    isSubmitting.value = false;
  }
};

// 複製機能がある場合の初期化
onMounted(() => {
  const route = useRoute();
  if (route.query.duplicate) {
    const duplicateData = localStorage.getItem(route.query.duplicate);
    if (duplicateData) {
      try {
        const parsedData = JSON.parse(duplicateData);
        if (parsedData.data && parsedData.expires > Date.now()) {
          // 複製データをロード
          machine.value = {
            ...parsedData.data,
            hostname: `${parsedData.data.hostname}`,
            id: '',
            parent_machine_id: parsedData.data.is_virtual ? parsedData.data.parent_machine_id : ''
          };
        } else {
          localStorage.removeItem(route.query.duplicate);
        }
      } catch (e) {
        console.error('Failed to parse duplicate data', e);
      }
    }
  }
});
</script>
<style scoped>
*{ text-transform: none !important; }
</style>