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
            <th style="width: 160px;">贡献者</th>
            <th>项目</th> <!-- 不限制宽度，让它自由撑开 -->
            <th style="width: 380px;">提交信息</th> <!-- 限制最大宽度 -->
            <th style="width: 100px; text-align: right;">变更</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in commits" :key="item.id">
            <td class="date-cell">{{ new Date(item.authoredAt).toLocaleString('zh-CN', { hour12: false }) }}</td>
            <td>
              <div class="person compact">
                <div class="avatar">{{ (item.user?.name || item.authorName)?.slice(0, 1) }}</div>
                <div class="truncate" style="max-width: 120px;">
                  <b class="truncate">{{ item.user?.name || item.authorName }}</b>
                  <small class="truncate">{{ item.authorEmail }}</small>
                </div>
              </div>
            </td>
            <td>
              <!-- 项目列：完整展示，允许换行 -->
              <div style="white-space: normal; min-width: 150px; line-height: 1.4;">
                <span class="tag">{{ item.project?.group?.name || '未分组' }}</span>
                <small class="block" style="color: #61708a; margin-top: 4px;">
                  {{ item.project?.pathWithNamespace }}
                </small>
              </div>
            </td>
            <td>
              <!-- 提交信息列：超长缩略 -->
              <a
                v-if="item.webUrl"
                :href="item.webUrl"
                target="_blank"
                rel="noopener"
                class="commit-title truncate"
                :title="item.title"
                style="max-width: 380px; display: flex;"
              >
                <code style="flex-shrink: 0;">{{ item.shortSha }}</code>
                <span class="truncate" style="margin: 0 4px;">{{ item.title }}</span>
                <ExternalLink :size="13" style="flex-shrink: 0;" />
              </a>
              <span v-else class="commit-title truncate" :title="item.title" style="max-width: 380px; display: flex;">
                <code style="flex-shrink: 0;">{{ item.shortSha }}</code>
                <span class="truncate" style="margin: 0 4px;">{{ item.title }}</span>
              </span>
            </td>
            <td style="text-align: right;">
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
/* 强制单行截断并显示省略号 */
.truncate {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  display: block;
}

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

/* 表格基础优化：允许单元格内的文字换行 */
.commit-table td {
  padding-top: 12px;
  padding-bottom: 12px;
}
</style>