<script setup>
import { computed, ref, onMounted } from 'vue'
import { ChevronLeft, ChevronRight, Search } from 'lucide-vue-next'
import { api, queryString } from '../api'
import FilterBar from '../components/FilterBar.vue'

const filterBarRef = ref(null)
const rows = ref([])
const total = ref(0)
const page = ref(1)
const filters = ref({})
const loading = ref(true) // 初始设为 true，防止首次加载瞬间显示“无数据”
const error = ref('')
const search = ref('')
const pageSize = 20

// 加载贡献者数据
async function load(next = 1) {
  page.value = next
  loading.value = true
  error.value = ''
  try {
    const response = await api(`/contributions${queryString({ ...filters.value, page: page.value, pageSize })}`)
    rows.value = response.items || [] // 兜底防止 null
    total.value = response.total || 0
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

// 应用筛选条件
function apply(nextFilters) {
  filters.value = nextFilters
  load(1)
}

// 过滤行数据
const filteredRows = computed(() =>
  rows.value.filter(x => `${x.name} ${x.email}`.toLowerCase().includes(search.value.toLowerCase())),
)

// 计算总页数
const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

// 计算净变化
function net(row) {
  return row.additions - row.deletions
}

// 首次挂载时，如果 FilterBar 没有自动触发，兜底加载一次
onMounted(() => {
  // 延迟 100ms 等 FilterBar 内部初始化（如果有的话）
  // 如果 FilterBar 没有默认触发，这里可以手动调一次
  setTimeout(() => {
    if (loading.value) {
      // 如果 100ms 后还在 loading，说明没触发请求，手动请求一次
      load(1)
    }
  }, 100)
})
</script>

<template>
  <FilterBar ref="filterBarRef" @change="apply" />
  <section class="panel table-panel">
    <div class="panel-head">
      <div>
        <h2>贡献者列表</h2>
        <p>每位贡献者按邮箱别名归并，可在数据源配置中维护映射。</p>
      </div>
      <div class="table-tools">
        <div class="search">
          <Search :size="16" />
          <input v-model="search" placeholder="筛选当前页成员" />
        </div>
        <span class="count-label">{{ total.toLocaleString() }} 位</span>
      </div>
    </div>
    <p v-if="error" class="error-banner">{{ error }}</p>
    <div class="table-scroll">
      <table>
        <thead>
          <tr>
            <th>贡献者</th>
            <th>覆盖项目</th>
            <th>提交</th>
            <th>新增行</th>
            <th>删除行</th>
            <th>净变化</th>
            <th>最近提交</th>
          </tr>
        </thead>
        <tbody>
          <!-- 增加 loading 时的占位行 -->
          <tr v-if="loading">
            <td colspan="7" class="empty-state">正在加载数据…</td>
          </tr>
          <tr v-else v-for="row in filteredRows" :key="row.email">
            <td>
              <div class="person">
                <div class="avatar">{{ row.name?.slice(0, 1) }}</div>
                <div>
                  <b>{{ row.name }}</b>
                  <small>{{ row.email }}</small>
                </div>
              </div>
            </td>
            <td>{{ row.projects }}</td>
            <td>{{ row.commitCount.toLocaleString() }}</td>
            <td class="positive">+{{ row.additions.toLocaleString() }}</td>
            <td class="negative">−{{ row.deletions.toLocaleString() }}</td>
            <td :class="net(row) >= 0 ? 'positive' : 'negative'">
              {{ net(row) >= 0 ? '+' : '' }}{{ net(row).toLocaleString() }}
            </td>
            <td>{{ row.lastCommit ? new Date(row.lastCommit).toLocaleDateString('zh-CN') : '—' }}</td>
          </tr>
          <tr v-if="!loading && !filteredRows.length">
            <td colspan="7" class="empty-state">当前条件下没有贡献者数据。</td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="pagination">
      <span>第 {{ page }} / {{ pages }} 页</span>
      <div>
        <button class="button ghost small" :disabled="page === 1 || loading" @click="load(page - 1)">
          <ChevronLeft :size="16" />
          上一页
        </button>
        <button class="button ghost small" :disabled="page >= pages || loading" @click="load(page + 1)">
          下一页
          <ChevronRight :size="16" />
        </button>
      </div>
    </div>
  </section>
</template>