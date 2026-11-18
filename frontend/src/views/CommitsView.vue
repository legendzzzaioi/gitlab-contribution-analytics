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
    commits.value = response.items || []
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
            <th style="width: 140px;">提交时间</th>
            <th style="width: 180px;">贡献者</th>
            <th style="width: 200px;">项目</th>
            <th>提交信息</th> <!-- 不设宽度，自动撑满剩余空间 -->
            <th style="width: 100px; text-align: right;">变更</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in commits" :key="item.id">
            <td class="date-cell">{{ new Date(item.authoredAt).toLocaleString('zh-CN', { hour12: false }) }}</td>
            <td>
              <div class="person compact">
                <div class="avatar">{{ (item.user?.name || item.authorName)?.slice(0, 1) }}</div>
                <div class="truncate" style="max-width: 130px;">
                  <b class="truncate">{{ item.user?.name || item.authorName }}</b>
                  <small class="truncate">{{ item.authorEmail }}</small>
                </div>
              </div>
            </td>
            <td>
              <div class="truncate" style="max-width: 180px;">
                <span class="tag">{{ item.project?.group?.name || '未分组' }}</span>
                <small class="block truncate">{{ item.project?.pathWithNamespace }}</small>
              </div>
            </td>
            <td>
              <!-- 提交信息列：限制最大宽度，超出缩略 -->
              <a
                v-if="item.webUrl"
                :href="item.webUrl"
                target="_blank"
                rel="noopener"
                class="commit-title truncate"
                :title="item.title"
                style="max-width: 100%; display: flex;"
              >
                <code style="flex-shrink: 0;">{{ item.shortSha }}</code>
                <span class="truncate" style="margin: 0 4px;">{{ item.title }}</span>
                <ExternalLink :size="13" style="flex-shrink: 0;" />
              </a>
              <span v-else class="commit-title truncate" :title="item.title" style="max-width: 100%; display: flex;">
                <code style="flex-shrink: 0;">{{ item.shortSha }}</code>
                <span class="truncate" style="margin: 0 4px;">{{ item.title }}</span>
              </span>
            </td>
            <td style="text-align: right;">
              <!-- 变更列：靠右对齐并保持在一行 -->
              <div style="display: flex; justify-content: flex-end; gap: 6px; font-family: 'DM Mono', monospace; font-weight: 600;">
                <span class="positive">+{{ item.additions }}</span>
                <span class="negative">−{{ item.deletions }}</span>
              </div>
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
          <ChevronLeft :size="16" /> 上一页
        </button>
        <button class="button ghost small" :disabled="page >= pages || loading" @click="load(page + 1)">
          下一页 <ChevronRight :size="16" />
        </button>
      </div>
    </div>
  </section>
</template>

<style scoped>
/* 强制文字单行截断并显示省略号的通用类 */
.truncate {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  display: block;
}

/* 覆盖原有的 commit-title 样式，让它适应 Flex 布局 */
.commit-title {
  text-decoration: none;
  color: #30466f;
  font-weight: 600;
  font-size: 13px;
  align-items: center;
}
.commit-title:hover {
  color: var(--brand);
}

/* 覆盖原本的 small 和 b，支持截断 */
.person small, .person b {
  display: block;
}
</style>
