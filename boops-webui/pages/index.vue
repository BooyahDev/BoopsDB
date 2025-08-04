<template>
    <v-container style="margin-top: 50px;">
        <h1>BoopsDB マシン検索</h1>

        <!-- Link to Register a New Machine -->
        <nuxt-link to="/machines/register" class="register-link">マシン登録</nuxt-link>

        <!-- Machine Search Form -->
        <section>
            <form @submit.prevent="searchMachines">
                <div>
                <label for="search">hostname, ip, memo, etc...</label>
                <label for="search">例: AMD製CPUが搭載されたマシンの検索 "amd -intel -qemu"</label>
                <label for="search">この場合 "AMD" という文字列を探すが、アーキテクチャに "amd64" があるため、「intel qemu」を除外します。</label>
                <input v-model="searchQuery" type="text" id="search" />
                </div>
                <button type="submit">検索</button>
            </form>

        <!-- Search Results -->
        </section>
    </v-container>

</template>

<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';

const router = useRouter();
const searchQuery = ref('');

async function searchMachines() {
  if (searchQuery.value.trim()) {
    // Redirect to the machines page with query
    router.push({ path: '/machines', query: { q: searchQuery.value } });
  } else {
    // If no query, redirect without any query parameter
    router.push({ path: '/machines' });
  }
}
</script>

<style scoped>
/* .container {
  max-width: 800px;
  margin: auto;
  padding: 2rem;
} */
/* h1, h2, h3, h4 {
  color: #333;
} */
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
