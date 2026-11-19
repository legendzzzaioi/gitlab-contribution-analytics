<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { CheckCircle2, Clock3, Database, Play, RefreshCw, TriangleAlert, XCircle } from 'lucide-vue-next'
import { api } from '../api'

const runs = ref([])
const status = ref({})
const loading = ref(false)
const message = ref('')
const error = ref('')
let timer

// 加载同步历史和状态
async function load() {
  try {
    const [history, current] = await Promise.all([
      api('/sync/runs'),
      api('/sync/status'),
    ])
    runs.value = history.items
    status.value = current
  } catch (err) {
    error.value = err.message
  }
}

// 开始同步
async function start(full) {
  loading.value = true
  message.value = ''
  error.value = ''
  try {
    const response = await api('/sync/run', {
      method: 'POST',
      body: JSON.stringify({ full }),
    })
    message.value = response.message
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

// 获取运行状态图标
function icon(run) {
  return run.status === 'success' ? CheckCircle2 : run.status === 'failed' ? XCircle : RefreshCw
}

// 获取运行状态标签
function label(run) {
  return run.status === 'success' ? '成功' : run.status === 'failed' ? '失败' : '进行中'
}

// 组件挂载时加载数据并设置定时器
onMounted(async () => {
  await load()
  timer = setInterval(load, 10000)
})

// 组件卸载时清除定时器
onBeforeUnmount(() => clearInterval(timer))
</script>

<template>
  <section class="sync-hero panel">
    <div>
      <p class="eyebrow">BACKGROUND COLLECTION</p>
      <h2>GitLab 数据同步</h2>
      <p>同步任务独立运行。报表查询只读本地 MySQL，不会在页面筛选时请求 GitLab。</p>
      <div class="sync-actions">
        <button class="button primary" :disabled="loading" @click="start(false)">
          <RefreshCw :size="16" :class="{ spinning: loading }" />
          立即增量同步
        </button>
        <button class="button secondary" :disabled="loading" @click="start(true)">
          <Play :size="16" />
          首次 / 全量同步
        </button>
      </div>
      <p v-if="message" class="success-banner">{{ message }}</p>
      <p v-if="error" class="error-banner">{{ error }}</p>
    </div>
    <div class="sync-status">
      <Database :size="28" />
      <div>
        <span>最近成功完成</span>
        <b>{{ status.lastSuccessAt ? new Date(status.lastSuccessAt).toLocaleString('zh-CN', { hour12: false }) : '尚无数据' }}</b>
      </div>
    </div>
  </section>

  <section class="panel table-panel">
    <div class="panel-head">
      <div>
        <h2>运行历史</h2>
        <p>自动任务与手动任务都会留存最近 50 次记录。</p>
      </div>
      <button class="button ghost small" @click="load">
        <RefreshCw :size="15" />
        刷新
      </button>
    </div>
    <div class="table-scroll">
      <table>
        <thead>
          <tr>
            <th>状态</th>
            <th>模式</th>
            <th>开始时间</th>
            <th>完成时间</th>
            <th>扫描项目</th>
            <th>写入提交</th>
            <th>错误信息</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="run in runs" :key="run.id">
            <td>
              <span class="run-status" :class="run.status">
                <component :is="icon(run)" :size="15" />
                {{ label(run) }}
              </span>
            </td>
            <td>{{ run.mode === 'full' ? '全量' : '增量' }}</td>
            <td>{{ new Date(run.startedAt).toLocaleString('zh-CN', { hour12: false }) }}</td>
            <td>{{ run.finishedAt ? new Date(run.finishedAt).toLocaleString('zh-CN', { hour12: false }) : '—' }}</td>
            <td>{{ run.projectsScanned }}</td>
            <td>{{ run.commitsSynced }}</td>
            <td class="error-cell">{{ run.errorMessage || '—' }}</td>
          </tr>
          <tr v-if="!runs.length">
            <td colspan="7" class="empty-state">还没有同步记录。先在数据源配置中保存 GitLab 连接。</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <section class="notice">
    <TriangleAlert :size="18" />
    <div>
      <b>全量同步提示</b>
      <p>首次同步会遍历所选群组及所有子组项目的默认分支提交。仓库历史很长时请安排在低峰期执行，并使用只读服务账号令牌。</p>
    </div>
  </section>
</template>