<template>
  <v-card class="console-card mb-6 pa-5">
    <div>
      <v-btn color="primary" @click="$emit('duplicate')" class="mr-2">
        <v-icon start>mdi-content-copy</v-icon>
        マシンを複製
      </v-btn>
    </div>
    <h2 class="text-h6 mb-4 mt-4">基本情報</h2>
    <div>最終接続: {{ formatDate(machine.last_alive) }}</div>
    <div>更新日時: {{ formatDate(machine.updated_at) }}</div>
    <div>登録日時: {{ formatDate(machine.created_at) }}</div>
    <v-table class="elevation-1">
      <tbody>
        <tr>
          <th width="20%">ID:</th>
          <td width="80%">
            {{ machine.id }}
            <v-btn
              icon
              variant="text"
              size="small"
              aria-label="値をコピー"
              @click="copyToClipboard(machine.id, 'machine-id')"
              class="ml-2"
            >
              <v-icon>
                {{ copiedItems['machine-id'] ? 'mdi-check' : 'mdi-content-copy' }}
              </v-icon>
            </v-btn>
          </td>
        </tr>
        <tr>
          <th>ホスト名:</th>
          <td>
            <template v-if="isEditingHostname">
              <div class="d-flex align-center">
                <v-text-field v-model="editableHostname" density="compact" hide-details class="mr-2" />
                <v-btn color="success" icon size="small" @click="saveHostname" :loading="isUpdatingHostname" class="mr-1">
                  <v-icon>mdi-check</v-icon>
                </v-btn>
                <v-btn color="error" icon size="small" @click="cancelEditHostname">
                  <v-icon>mdi-close</v-icon>
                </v-btn>
              </div>
            </template>
            <template v-else>
              {{ machine.hostname }}
              <v-btn
                icon
                variant="text"
                size="small"
                aria-label="値をコピー"
              @click="copyToClipboard(machine.hostname, 'hostname')"
                class="ml-1 mr-1"
              >
                <v-icon>
                  {{ copiedItems['hostname'] ? 'mdi-check' : 'mdi-content-copy' }}
                </v-icon>
              </v-btn>
              <v-btn icon variant="text" size="small" aria-label="項目を編集" @click="enableEditHostname">
                <v-icon>mdi-pencil</v-icon>
              </v-btn>
            </template>
          </td>
        </tr>
        <tr>
          <th>OS:</th>
          <td>{{ machine.os_name || '未設定' }}</td>
        </tr>
        <tr>
          <th>CPU:</th>
          <td>{{ machine.cpu_info || '未設定' }}</td>
        </tr>
        <tr>
          <th>CPU アーキテクチャ:</th>
          <td>{{ machine.cpu_arch || '未設定' }}</td>
        </tr>
        <tr>
          <th>メモリー容量:</th>
          <td>{{ machine.memory_size || '未設定' }}</td>
        </tr>
        <tr>
          <th>ディスク情報:</th>
          <td>{{ machine.disk_info || '未設定' }}</td>
        </tr>
        <tr>
          <th>仮想マシン:</th>
          <td>
            <template v-if="isEditingVmStatus">
              <div>
                <v-checkbox v-model="vmStatusEdit.is_virtual" label="仮想マシン" hide-details density="compact" @change="handleVmStatusChange" />
                <div v-if="vmStatusEdit.is_virtual" class="mt-2">
                  <v-text-field v-model="vmStatusEdit.parent_machine_id" label="親マシン UUID" density="compact" hide-details />
                </div>
                <div class="mt-2">
                  <v-btn color="success" size="small" @click="saveVmStatus" :loading="isUpdatingVmStatus" class="mr-2">
                    保存
                  </v-btn>
                  <v-btn color="error" size="small" @click="cancelEditVmStatus">
                    キャンセル
                  </v-btn>
                </div>
              </div>
            </template>
            <template v-else>
              {{ machine.is_virtual ? 'Yes' : 'No' }}
              <v-btn icon variant="text" size="small" aria-label="項目を編集" @click="enableEditVmStatus" class="ml-1">
                <v-icon>mdi-pencil</v-icon>
              </v-btn>
            </template>
          </td>
        </tr>
        <tr>
          <th>用途:</th>
          <td>
            <template v-if="isEditingPurpose">
              <div class="d-flex align-center">
                <v-text-field v-model="editablePurpose" density="compact" hide-details class="mr-2" />
                <v-btn color="success" icon size="small" @click="updatePurpose" :loading="isUpdatingPurpose" class="mr-1">
                  <v-icon>mdi-check</v-icon>
                </v-btn>
                <v-btn color="error" icon size="small" @click="cancelEditPurpose">
                  <v-icon>mdi-close</v-icon>
                </v-btn>
              </div>
            </template>
            <template v-else>
              {{ machine.purpose || '未設定' }}
              <v-btn icon variant="text" size="small" aria-label="項目を編集" @click="enableEditPurpose" class="ml-1">
                <v-icon>mdi-pencil</v-icon>
              </v-btn>
            </template>
          </td>
        </tr>
        <tr v-if="machine.is_virtual">
          <th>親マシン UUID:</th>
          <td>
            <template v-if="isEditingParentId">
              <div class="d-flex align-center">
                <v-text-field v-model="editableParentId" density="compact" hide-details class="mr-2" />
                <v-btn color="success" icon size="small" @click="updateParentId" :loading="isUpdatingParentId" class="mr-1">
                  <v-icon>mdi-check</v-icon>
                </v-btn>
                <v-btn color="error" icon size="small" @click="cancelEditParentId">
                  <v-icon>mdi-close</v-icon>
                </v-btn>
              </div>
            </template>
            <template v-else>
              {{ machine.parent_machine_id || '未設定' }}
              <v-btn
                icon
                variant="text"
                size="small"
                aria-label="値をコピー"
              @click="copyToClipboard(machine.parent_machine_id, 'parent_machine-id')"
                class="ml-2"
              >
                <v-icon>
                  {{ copiedItems['parent_machine-id'] ? 'mdi-check' : 'mdi-content-copy' }}
                </v-icon>
              </v-btn>
            </template>
          </td>
        </tr>
        <tr v-if="machine.is_virtual && machine.parent_machine_id">
          <th>親マシン:</th>
          <td>
            <v-btn variant="text" :to="`/machines/${machine.parent_machine_id}`" color="primary">
              {{ machine.parentHostname || 'Unknown' }}
              <v-icon end>mdi-open-in-new</v-icon>
            </v-btn>
          </td>
        </tr>
        <tr>
          <th>メモ:</th>
          <td>
            <template v-if="isEditingMemo">
              <div>
                <v-textarea v-model="editableMemo" rows="3" density="compact" hide-details />
                <div class="mt-2">
                  <v-btn color="success" size="small" @click="updateMemo" :loading="isUpdatingMemo" class="mr-2">
                    保存
                  </v-btn>
                  <v-btn color="error" size="small" @click="cancelEditMemo">
                    キャンセル
                  </v-btn>
                </div>
              </div>
            </template>
            <template v-else>
              <div v-if="machine.memo" class="memo-markdown" v-html="renderedMemo"></div>
              <span v-else>N/A</span>
              <v-btn icon variant="text" size="small" aria-label="項目を編集" @click="enableEditMemo" class="ml-1">
                <v-icon>mdi-pencil</v-icon>
              </v-btn>
            </template>
          </td>
        </tr>
      </tbody>
    </v-table>
  </v-card>
</template>

<script setup>
import { ref, computed } from 'vue';
import { useClipboard } from '@/composables/useClipboard';
import { useDateFormatter } from '@/composables/useDateFormatter';
import { useMachineApi } from '@/composables/useMachineApi';

const props = defineProps({
  machine: {
    type: Object,
    required: true
  }
});

const emit = defineEmits(['update:machine', 'duplicate']);

const { copyToClipboard, copiedItems } = useClipboard();
const { formatDate } = useDateFormatter();
const { updateMachineField } = useMachineApi();

const renderedMemo = computed(() => renderMarkdown(props.machine.memo || ''));

// Editable fields
const editableHostname = ref('');
const editablePurpose = ref('');
const editableParentId = ref('');
const editableMemo = ref('');

// Edit states
const isEditingHostname = ref(false);
const isEditingPurpose = ref(false);
const isEditingParentId = ref(false);
const isEditingMemo = ref(false);
const isEditingVmStatus = ref(false);

// Loading states
const isUpdatingHostname = ref(false);
const isUpdatingPurpose = ref(false);
const isUpdatingParentId = ref(false);
const isUpdatingMemo = ref(false);
const isUpdatingVmStatus = ref(false);

// VM status editing
const vmStatusEdit = ref({
  is_virtual: false,
  parent_machine_id: null
});

// Hostname editing
const enableEditHostname = () => {
  editableHostname.value = props.machine.hostname;
  isEditingHostname.value = true;
};

const cancelEditHostname = () => {
  isEditingHostname.value = false;
  editableHostname.value = '';
};

const saveHostname = async () => {
  isUpdatingHostname.value = true;
  try {
    const updatedMachine = await updateMachineField(props.machine.id, 'hostname', editableHostname.value);
    emit('update:machine', updatedMachine);
    isEditingHostname.value = false;
  } catch (error) {
    alert(`Failed to update hostname: ${error.message}`);
  } finally {
    isUpdatingHostname.value = false;
  }
};

// Purpose editing
const enableEditPurpose = () => {
  editablePurpose.value = props.machine.purpose || '';
  isEditingPurpose.value = true;
};

const cancelEditPurpose = () => {
  isEditingPurpose.value = false;
  editablePurpose.value = '';
};

const updatePurpose = async () => {
  isUpdatingPurpose.value = true;
  try {
    const updatedMachine = await updateMachineField(props.machine.id, 'purpose', editablePurpose.value);
    emit('update:machine', updatedMachine);
    isEditingPurpose.value = false;
  } catch (error) {
    alert(`Failed to update purpose: ${error.message}`);
  } finally {
    isUpdatingPurpose.value = false;
  }
};

// Parent ID editing
const enableEditParentId = () => {
  editableParentId.value = props.machine.parent_machine_id || '';
  isEditingParentId.value = true;
};

const cancelEditParentId = () => {
  isEditingParentId.value = false;
  editableParentId.value = '';
};

const updateParentId = async () => {
  isUpdatingParentId.value = true;
  try {
    const updatedMachine = await updateMachineField(props.machine.id, 'parent_machine_id', editableParentId.value);
    emit('update:machine', updatedMachine);
    isEditingParentId.value = false;
  } catch (error) {
    alert(`Failed to update parent machine ID: ${error.message}`);
  } finally {
    isUpdatingParentId.value = false;
  }
};

// Memo editing
const enableEditMemo = () => {
  editableMemo.value = props.machine.memo || '';
  isEditingMemo.value = true;
};

const cancelEditMemo = () => {
  isEditingMemo.value = false;
  editableMemo.value = '';
};

const updateMemo = async () => {
  isUpdatingMemo.value = true;
  try {
    const updatedMachine = await updateMachineField(props.machine.id, 'memo', editableMemo.value);
    emit('update:machine', updatedMachine);
    isEditingMemo.value = false;
  } catch (error) {
    alert(`Failed to update memo: ${error.message}`);
  } finally {
    isUpdatingMemo.value = false;
  }
};

// VM Status editing
const enableEditVmStatus = () => {
  vmStatusEdit.value = {
    is_virtual: props.machine.is_virtual,
    parent_machine_id: props.machine.parent_machine_id
  };
  isEditingVmStatus.value = true;
};

const cancelEditVmStatus = () => {
  isEditingVmStatus.value = false;
};

const handleVmStatusChange = () => {
  if (!vmStatusEdit.value.is_virtual) {
    vmStatusEdit.value.parent_machine_id = null;
  }
};

const saveVmStatus = async () => {
  isUpdatingVmStatus.value = true;
  try {
    if (vmStatusEdit.value.parent_machine_id === '') {
      vmStatusEdit.value.is_virtual = false;
      vmStatusEdit.value.parent_machine_id = null;
    }

    const updatedMachine = await updateMachineField(props.machine.id, 'vm_status', vmStatusEdit.value);
    emit('update:machine', updatedMachine);
    isEditingVmStatus.value = false;
  } catch (error) {
    alert(`Failed to update VM status: ${error.message}`);
  } finally {
    isUpdatingVmStatus.value = false;
  }
};

const renderMarkdown = (source) => {
  const blocks = [];
  let listItems = [];

  const flushList = () => {
    if (listItems.length > 0) {
      blocks.push(`<ul>${listItems.map(item => `<li>${renderInlineMarkdown(item)}</li>`).join('')}</ul>`);
      listItems = [];
    }
  };

  for (const rawLine of source.replace(/\r\n/g, '\n').split('\n')) {
    const line = rawLine.trimEnd();
    const heading = line.match(/^(#{1,3})\s+(.+)$/);
    const listItem = line.match(/^[-*]\s+(.+)$/);

    if (heading) {
      flushList();
      const level = heading[1].length + 2;
      blocks.push(`<h${level}>${renderInlineMarkdown(heading[2])}</h${level}>`);
    } else if (listItem) {
      listItems.push(listItem[1]);
    } else if (line.trim() === '') {
      flushList();
      blocks.push('<br>');
    } else {
      flushList();
      blocks.push(`<p>${renderInlineMarkdown(line)}</p>`);
    }
  }

  flushList();
  return blocks.join('');
};

const renderInlineMarkdown = (source) => {
  let html = escapeHtml(source);
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>');
  html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  html = html.replace(/\*([^*]+)\*/g, '<em>$1</em>');
  html = html.replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
  html = html.replace(/(^|[\s>])(https?:\/\/[^\s<]+)/g, '$1<a href="$2" target="_blank" rel="noopener noreferrer">$2</a>');
  return html;
};

const escapeHtml = (source) => source
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;')
  .replace(/'/g, '&#39;');
</script>

<style scoped>
* { text-transform: none !important; }

.memo-markdown {
  display: inline-block;
  max-width: 100%;
  vertical-align: top;
  word-break: break-word;
}

.memo-markdown :deep(p) {
  margin: 0 0 0.5rem;
}

.memo-markdown :deep(p:last-child),
.memo-markdown :deep(ul:last-child) {
  margin-bottom: 0;
}

.memo-markdown :deep(ul) {
  margin: 0 0 0.5rem 1.25rem;
  padding: 0;
}

.memo-markdown :deep(h3),
.memo-markdown :deep(h4),
.memo-markdown :deep(h5) {
  margin: 0.4rem 0 0.25rem;
  font-size: 1rem;
  font-weight: 700;
}

.memo-markdown :deep(code) {
  background: rgba(var(--v-theme-on-surface), 0.08);
  border-radius: 4px;
  padding: 0.1rem 0.25rem;
}
</style>
