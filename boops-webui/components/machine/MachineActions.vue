<template>
  <div>
    <v-btn color="error" @click="showDeleteDialog = true">
      <v-icon start>mdi-delete</v-icon>
      Machine Delete
    </v-btn>

    <!-- Machine delete confirmation dialog -->
    <v-dialog v-model="showDeleteDialog" max-width="500">
      <v-card>
        <v-card-title>Confirm Machine Deletion</v-card-title>
        <v-card-text>
          <p>本当にこのマシンを削除しますか？</p>
          <p class="text-error">この操作は元に戻せません！</p>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn color="error" @click="deleteMachine" :loading="isDeleting">
            削除
          </v-btn>
          <v-btn color="secondary" @click="showDeleteDialog = false">
            キャンセル
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { useMachineApi } from '@/composables/useMachineApi';

const props = defineProps({
  machineId: {
    type: String,
    required: true
  }
});

const router = useRouter();
const { deleteMachine: deleteMachineApi } = useMachineApi();

const showDeleteDialog = ref(false);
const isDeleting = ref(false);

const deleteMachine = async () => {
  isDeleting.value = true;
  try {
    await deleteMachineApi(props.machineId);
    router.push('/');
  } catch (err) {
    alert(err.message);
  } finally {
    isDeleting.value = false;
    showDeleteDialog.value = false;
  }
};
</script>
