<template>
    <v-container style="margin-top: 50px;">
        <h1>BoopsDB マシン検索</h1>

        <!-- Link to Register a New Machine -->
        <nuxt-link to="/machines/register" class="register-link">マシン登録</nuxt-link>

        <!-- Search Statistics -->
        <v-card v-if="searchStats" class="mb-4" elevation="2">
            <v-card-title>統計情報</v-card-title>
            <v-card-text>
                <v-row>
                    <v-col cols="6" md="3">
                        <v-chip color="primary" label>
                            総マシン数: {{ searchStats.total_machines }}
                        </v-chip>
                    </v-col>
                    <v-col cols="6" md="3">
                        <v-chip color="green" label>
                            仮想: {{ searchStats.virtual_machines }}
                        </v-chip>
                    </v-col>
                    <v-col cols="6" md="3">
                        <v-chip color="blue" label>
                            物理: {{ searchStats.physical_machines }}
                        </v-chip>
                    </v-col>
                    <v-col cols="6" md="3">
                        <v-chip color="orange" label>
                            1日以内: {{ searchStats.alive_last_day }}
                        </v-chip>
                    </v-col>
                </v-row>
            </v-card-text>
        </v-card>

        <!-- Advanced Search Help -->
        <v-expansion-panels v-model="helpPanel" class="mb-4">
            <v-expansion-panel>
                <v-expansion-panel-title>
                    <v-icon start>mdi-help-circle</v-icon>
                    高度な検索機能
                </v-expansion-panel-title>
                <v-expansion-panel-text>
                    <v-row>
                        <v-col cols="12" md="6">
                            <h4>検索方法</h4>
                            <ul>
                                <li><strong>フィールド指定:</strong> hostname:server01, os:ubuntu</li>
                                <li><strong>IP検索:</strong> 192.168.1.100 または 192.168.1</li>
                                <li><strong>CIDR検索:</strong> 192.168.1.0/24</li>
                                <li><strong>MAC検索:</strong> 00:1A:2B:3C:4D:5E</li>
                                <li><strong>マシンタイプ:</strong> virtual または physical</li>
                                <li><strong>生存時間:</strong> alive:1h, alive:1d, alive:30m</li>
                                <li><strong>除外検索:</strong> -test, -old</li>
                            </ul>
                        </v-col>
                        <v-col cols="12" md="6">
                            <h4>検索例</h4>
                            <v-chip-group column>
                                <v-chip v-for="example in searchExamples" :key="example" 
                                       @click="setSearchExample(example)" size="small">
                                    {{ example }}
                                </v-chip>
                            </v-chip-group>
                        </v-col>
                    </v-row>
                </v-expansion-panel-text>
            </v-expansion-panel>
        </v-expansion-panels>

        <!-- Search Suggestions -->
        <v-card v-if="suggestions" class="mb-4">
            <v-card-title>クイック検索</v-card-title>
            <v-card-text>
                <div v-if="suggestions.hostnames.length > 0" class="mb-3">
                    <h6>ホスト名:</h6>
                    <v-chip-group>
                        <v-chip v-for="hostname in suggestions.hostnames.slice(0, 10)" 
                               :key="hostname" size="small"
                               @click="setSearchQuery('hostname:' + hostname)">
                            {{ hostname }}
                        </v-chip>
                    </v-chip-group>
                </div>
                <div v-if="suggestions.osNames.length > 0" class="mb-3">
                    <h6>OS:</h6>
                    <v-chip-group>
                        <v-chip v-for="os in suggestions.osNames" 
                               :key="os" size="small"
                               @click="setSearchQuery('os:' + os)">
                            {{ os }}
                        </v-chip>
                    </v-chip-group>
                </div>
                <div v-if="suggestions.purposes.length > 0" class="mb-3">
                    <h6>用途:</h6>
                    <v-chip-group>
                        <v-chip v-for="purpose in suggestions.purposes" 
                               :key="purpose" size="small"
                               @click="setSearchQuery('purpose:' + purpose)">
                            {{ purpose }}
                        </v-chip>
                    </v-chip-group>
                </div>
            </v-card-text>
        </v-card>

        <!-- Machine Search Form -->
        <section>
            <form @submit.prevent="searchMachines">
                <v-text-field
                    v-model="searchQuery"
                    label="検索クエリを入力..."
                    hint="ホスト名、IP、メモなど..."
                    persistent-hint
                    variant="outlined"
                    clearable
                    append-inner-icon="mdi-magnify"
                    @click:append-inner="searchMachines"
                />
                
                <!-- Sort and Pagination Controls -->
                <v-row class="mt-2">
                    <v-col cols="12" md="4">
                        <v-select
                            v-model="sortBy"
                            :items="sortOptions"
                            label="ソート"
                            variant="outlined"
                            density="compact"
                        />
                    </v-col>
                    <v-col cols="12" md="4">
                        <v-select
                            v-model="sortOrder"
                            :items="sortOrderOptions"
                            label="順序"
                            variant="outlined"
                            density="compact"
                        />
                    </v-col>
                    <v-col cols="12" md="4">
                        <v-select
                            v-model="pageSize"
                            :items="pageSizeOptions"
                            label="表示件数"
                            variant="outlined"
                            density="compact"
                        />
                    </v-col>
                </v-row>

                <v-btn type="submit" color="primary" size="large" block>
                    <v-icon start>mdi-magnify</v-icon>
                    検索
                </v-btn>
            </form>
        </section>
    </v-container>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { apiBaseUrl } from '@/apiConfig';

const router = useRouter();
const searchQuery = ref('');
const searchStats = ref(null);
const suggestions = ref(null);
const helpPanel = ref([]);

// Search configuration
const sortBy = ref('hostname');
const sortOrder = ref('asc');
const pageSize = ref(50);

const sortOptions = [
    { title: 'ホスト名', value: 'hostname' },
    { title: '最終接続', value: 'last_alive' },
    { title: '作成日', value: 'created_at' },
    { title: '更新日', value: 'updated_at' },
    { title: 'OS', value: 'os_name' },
    { title: '用途', value: 'purpose' }
];

const sortOrderOptions = [
    { title: '昇順', value: 'asc' },
    { title: '降順', value: 'desc' }
];

const pageSizeOptions = [
    { title: '25件', value: 25 },
    { title: '50件', value: 50 },
    { title: '100件', value: 100 },
    { title: '200件', value: 200 }
];

const searchExamples = [
    'virtual',
    'physical',
    'ubuntu -test',
    'hostname:web',
    'alive:1d',
    'os:windows',
    '192.168.1',
    'alive:1h virtual'
];

onMounted(async () => {
    await fetchSearchSuggestions();
});

async function fetchSearchSuggestions() {
    try {
        const response = await fetch(`${apiBaseUrl}/search/suggestions`);
        if (response.ok) {
            const data = await response.json();
            suggestions.value = data.suggestions;
            searchStats.value = data.statistics;
        }
    } catch (error) {
        console.error('Failed to fetch search suggestions:', error);
    }
}

function setSearchQuery(query) {
    searchQuery.value = query;
}

function setSearchExample(example) {
    searchQuery.value = example;
}

async function searchMachines() {
    const query = new URLSearchParams();
    
    if (searchQuery.value.trim()) {
        query.append('q', searchQuery.value);
    }
    
    query.append('sort', sortBy.value);
    query.append('order', sortOrder.value);
    query.append('limit', pageSize.value.toString());
    
    // Redirect to the machines page with query
    router.push({ path: '/machines', query: Object.fromEntries(query) });
}
</script>

<style scoped>
.register-link {
    display: inline-block;
    background-color: #28a745;
    color: white;
    padding: 0.5rem 1rem;
    border-radius: 4px;
    text-decoration: none;
    margin-bottom: 1rem;
}

.register-link:hover {
    background-color: #218838;
}

h6 {
    font-weight: bold;
    color: #666;
    margin-bottom: 8px;
}
</style>
