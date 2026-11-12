<script setup>
import { computed, ref } from 'vue'
import { ChevronLeft, ChevronRight, Download, ExternalLink } from 'lucide-vue-next'
import { api, queryString } from '../api'
import FilterBar from '../components/FilterBar.vue'

const commits = ref([])
const total = ref(0)
const page = ref(1)
const filters = ref({})
const loading = ref(false)
const error = ref('')
const pageSize = 30

// 加载提交数据
async function load(next = 1) {
  page.value = next
  loading.value = true
  error.value = ''
  try {
    const response = await api(`/commits${queryString({ ...filters.value, page: page.value, pageSize })}`)
    commits.value = response.items
    total.value = response.total
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

// 计算总页数
const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

// 导出CSV
async function exportCsv() {
  try {
    const response = await fetch(`/api/export/commits.csv${queryString(filters.value)}`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('analytics_token')}` },
    })
    if (!response.ok) throw new Error('导出失败，请重新登录后再试')
    const url = URL.createObjectURL(await response.blob())
    const link = document.createElement('a')
    link.href = url
    link.download = 'gitlab-contributions.csv'
    link.click()
    URL.revokeObjectURL(url)
  } catch (err) {
    error.value = err.message
  }
}
</script>

<template>
  <FilterBar @change="apply" />
  <section class="panel table-panel">
    <div class="panel-head">
      <div>
        <h2>提交明细</h2>
        <p>最多可导出符合当前筛选的前 10,000 条提交。</p>
      </div>
      <button class="button secondary" @click="exportCsv">
        <Download :size="16" />
        导出 CSV
      </button>
    </div>
    <p v-if="error" class="error-banner">{{ error }}</p>
    <div class="table-scroll">
      <table class="commit-table">
        <thead>
          <tr>
            <th>提交时间</th>
            <th>贡献者</th>
            <th>项目</th>
            <th>提交信息</th>
            <th>变更</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in commits" :key="item.id">
            <td class="date-cell">{{ new Date(item.authoredAt).toLocaleString('zh-CN', { hour12: false }) }}</td>
            <td>
              <div class="person compact">
                <div class="avatar">{{ (item.user?.name || item.authorName)?.slice(0, 1) }}</div>
                <div>
                  <b>{{ item.user?.name || item.authorName }}</b>
                  <small>{{ item.authorEmail }}</small>
                </div>
              </div>
            </td>
            <td>
              <span class="tag">{{ item.project?.group?.name || '未分组' }}</span>
              <small class="block">{{ item.project?.pathWithNamespace }}</small>
            </td>
            <td>
              <a
                v-if="item.webUrl"
                :href="item.webUrl"
                target="_blank"
                rel="noopener"
                class="commit-title"
              >
                <code>{{ item.shortSha }}</code>{{ item.title }}<ExternalLink :size="13" />
              </a>
              <span v-else class="commit-title">
                <code>{{ item.shortSha }}</code>{{ item.title }}
              </span>
            </td>
            <td>
              <span class="positive">+{{ item.additions }}</span>
              <span class="negative">−{{ item.deletions }}</span>
            </td>
          </tr>
          <tr v-if="!loading && !commits.length">
            <td colspan="5" class="empty-state">当前条件下没有提交记录。</td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="pagination">
      <span>共 {{ total.toLocaleString() }} 条 · 第 {{ page }} / {{ pages }} 页</span>
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