<script setup>
import { computed, ref } from 'vue'
import { Activity, ArrowDownRight, ArrowUpRight, FolderGit2, Users } from 'lucide-vue-next'
import { api, queryString } from '../api'
import FilterBar from '../components/FilterBar.vue'
import BaseChart from '../components/BaseChart.vue'
import StatCard from '../components/StatCard.vue'

const loading = ref(false)
const error = ref('')
const data = ref({ summary: {}, trend: [], topContributors: [], topProjects: [] })

// 加载仪表板数据
async function load(filters) {
  loading.value = true
  error.value = ''
  try {
    const res = await api(`/dashboard${queryString(filters)}`)
    // 兜底：防止后端首次无数据时返回 null 导致页面崩溃
    data.value = {
      summary: res?.summary || {},
      trend: res?.trend || [],
      topContributors: res?.topContributors || [],
      topProjects: res?.topProjects || []
    }
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

// 趋势图配置
const trendOption = computed(() => ({
  tooltip: { trigger: 'axis' },
  grid: { left: 42, right: 18, top: 30, bottom: 35 },
  legend: { data: ['新增行', '删除行'], bottom: 0 },
  xAxis: {
    type: 'category',
    boundaryGap: false,
    data: data.value.trend.map(x => x.date.slice(5)),
  },
  yAxis: {
    type: 'value',
    splitLine: { lineStyle: { color: '#e8edf5' } },
  },
  series: [
    {
      name: '新增行',
      type: 'line',
      smooth: true,
      showSymbol: false,
      lineStyle: { color: '#3366e8', width: 3 },
      areaStyle: { color: 'rgba(51,102,232,.10)' },
      data: data.value.trend.map(x => x.additions),
    },
    {
      name: '删除行',
      type: 'line',
      smooth: true,
      showSymbol: false,
      lineStyle: { color: '#d6703d', width: 2 },
      data: data.value.trend.map(x => x.deletions),
    },
  ],
}))

// 贡献者图表配置
const peopleOption = computed(() => ({
  tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
  grid: { left: 42, right: 20, top: 12, bottom: 34 },
  xAxis: {
    type: 'value',
    splitLine: { lineStyle: { color: '#e8edf5' } },
  },
  yAxis: {
    type: 'category',
    data: [...data.value.topContributors].reverse().map(x => x.name),
    axisTick: { show: false },
  },
  series: [
    {
      type: 'bar',
      data: [...data.value.topContributors].reverse().map(x => x.additions),
      itemStyle: { color: '#2d9d8f', borderRadius: [0, 4, 4, 0] },
      barMaxWidth: 22,
    },
  ],
}))
</script>

<template>
  <FilterBar @change="load" />
  <p v-if="error" class="error-banner">{{ error }}</p>

  <section class="stat-grid" :aria-busy="loading">
    <!-- 兜底：加上 ?. 防止 summary 为 undefined -->
    <StatCard label="提交次数(当前筛选条件)" :value="data.summary?.commits" note="所有分支，已排除合并提交" />
    <StatCard label="新增代码行(当前筛选条件)" :value="data.summary?.additions" suffix="行" tone="green" note="提交统计的 additions 汇总" />
    <StatCard label="删除代码行(当前筛选条件)" :value="data.summary?.deletions" suffix="行" tone="orange" note="提交统计的 deletions 汇总" />
    <StatCard label="活跃贡献者(当前筛选条件)" :value="data.summary?.contributors" tone="purple" :note="`${data.summary?.projects || 0} 个活跃项目`" />
  </section>

  <section class="content-grid dashboard-grid">
    <article class="panel chart-panel">
      <div class="panel-head">
        <div>
          <h2>代码变更趋势</h2>
          <p>按提交作者日期汇总，单位：行</p>
        </div>
        <Activity :size="19" />
      </div>
      <BaseChart :option="trendOption" />
    </article>
    <article class="panel chart-panel">
      <div class="panel-head">
        <div>
          <h2>贡献者新增行排行</h2>
          <p>当前筛选条件下前 8 位</p>
        </div>
        <Users :size="19" />
      </div>
      <BaseChart :option="peopleOption" />
    </article>
  </section>

  <section class="content-grid dashboard-grid">
    <article class="panel">
      <div class="panel-head">
        <div>
          <h2>贡献者概览</h2>
          <p>按新增行排序</p>
        </div>
        <RouterLink class="text-link" to="/contributions">查看全部</RouterLink>
      </div>
      <div class="rank-list">
        <!-- 兜底：数组安全遍历 -->
        <div v-for="(item, index) in data.topContributors" :key="item.email" class="rank-row">
          <span class="rank-no">{{ String(index + 1).padStart(2, '0') }}</span>
          <div class="avatar">{{ item.name?.slice(0, 1) }}</div>
          <div class="rank-name">
            <b>{{ item.name }}</b>
            <small>{{ item.projects }} 个项目 · {{ item.commitCount }} 次提交</small>
          </div>
          <strong class="positive">+{{ item.additions.toLocaleString() }}</strong>
          <span class="negative">−{{ item.deletions.toLocaleString() }}</span>
        </div>
        <p v-if="!data.topContributors.length" class="empty-state">当前筛选没有可展示的提交数据。</p>
      </div>
    </article>
    <article class="panel">
      <div class="panel-head">
        <div>
          <h2>项目贡献分布</h2>
          <p>按新增行排序</p>
        </div>
        <FolderGit2 :size="19" />
      </div>
      <div class="project-list">
        <!-- 兜底：数组安全遍历 -->
        <div v-for="item in data.topProjects" :key="item.projectId" class="project-row">
          <div>
            <b>{{ item.name }}</b>
            <small>{{ item.groupName }} · {{ item.commitCount }} 次提交 · {{ item.contributors }} 位成员</small>
          </div>
          <div>
            <strong>+{{ item.additions.toLocaleString() }}</strong>
            <span>−{{ item.deletions.toLocaleString() }}</span>
          </div>
        </div>
        <p v-if="!data.topProjects.length" class="empty-state">当前筛选没有项目数据。</p>
      </div>
    </article>
  </section>

  <p class="metric-note">
    <ArrowUpRight :size="15" />
    新增/删除行数来自 GitLab Commit Stats，是代码变更规模信号，不等同于个人绩效。
  </p>
</template>