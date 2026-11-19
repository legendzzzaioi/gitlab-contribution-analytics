<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { Filter, RotateCcw } from 'lucide-vue-next'
import { api } from '../api'

const emit = defineEmits(['change'])
const loading = ref(true)
const groups = ref([])
const projects = ref([])
const users = ref([])
const today = new Date().toISOString().slice(0, 10)
const monthAgo = new Date(Date.now() - 29 * 86400000).toISOString().slice(0, 10)
const filters = reactive({
  from: monthAgo,
  to: today,
  groupId: '',
  projectId: '',
  userId: '',
})
// 加载筛选选项
async function loadOptions() {
  loading.value = true
  try {
    const [groupResult, projectResult, userResult] = await Promise.all([
      api('/groups'),
      api('/projects'),
      api('/users'),
    ])
    groups.value = groupResult.items || []
    projects.value = projectResult.items || []
    users.value = userResult.items || []
  } finally {
    loading.value = false
  }
}

// 应用筛选条件
function apply() {
  emit('change', { ...filters })
}

// 重置筛选条件
function reset() {
  Object.assign(filters, {
    from: monthAgo,
    to: today,
    groupId: '',
    projectId: '',
    userId: '',
  })
  apply()
}

// 监听群组变化，清空项目筛选
watch(
  () => filters.groupId,
  () => {
    filters.projectId = ''
  },
)

// 组件挂载时加载选项并应用默认筛选
onMounted(async () => {
  await loadOptions()
  apply()
})

// 暴露 reloadOptions 方法，供父组件调用
defineExpose({ reloadOptions: loadOptions })
</script>

<template>
  <section class="filter-bar" aria-label="数据筛选器">
    <div class="filter-field">
      <label for="from">开始日期</label>
      <input id="from" v-model="filters.from" type="date" />
    </div>
    <div class="filter-field">
      <label for="to">结束日期</label>
      <input id="to" v-model="filters.to" type="date" />
    </div>
    <div class="filter-field">
      <label for="group">群组</label>
      <select id="group" v-model="filters.groupId" :disabled="loading">
        <option value="">全部群组</option>
        <option v-for="item in groups" :key="item.gitlabId" :value="item.gitlabId">{{ item.name }}</option>
      </select>
    </div>
    <div class="filter-field">
      <label for="project">项目</label>
      <select id="project" v-model="filters.projectId" :disabled="loading">
        <option value="">全部项目</option>
        <option
          v-for="item in projects.filter(p => !filters.groupId || p.group?.rootGitlabId === Number(filters.groupId))"
          :key="item.id"
          :value="item.id"
        >
          {{ item.pathWithNamespace }}
        </option>
      </select>
    </div>
    <div class="filter-field">
      <label for="user">贡献者</label>
      <select id="user" v-model="filters.userId" :disabled="loading">
        <option value="">全部贡献者</option>
        <option v-for="item in users" :key="item.id" :value="item.id">{{ item.name }} · {{ item.email }}</option>
      </select>
    </div>
    <div class="filter-actions">
      <button class="button primary" :disabled="loading" @click="apply">
        <Filter :size="16" />
        应用筛选
      </button>
      <button class="button ghost" title="重置筛选" @click="reset">
        <RotateCcw :size="16" />
      </button>
    </div>
  </section>
</template>