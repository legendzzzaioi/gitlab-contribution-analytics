<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { BarChart3, UsersRound, GitCommitHorizontal, RefreshCw, Settings, LogOut, Menu } from 'lucide-vue-next'

const route = useRoute()
const router = useRouter()
const isLogin = computed(() => route.path === '/login')
const links = [
  { to: '/', label: '总览', icon: BarChart3 },
  { to: '/contributions', label: '贡献者', icon: UsersRound },
  { to: '/commits', label: '提交明细', icon: GitCommitHorizontal },
  { to: '/permissions/tree', label: '群组权限', icon: UsersRound },
  { to: '/permissions/user', label: '人员权限', icon: UsersRound },
  { to: '/sync', label: '同步中心', icon: RefreshCw },
  { to: '/settings', label: '数据源配置', icon: Settings }
]
function logout() { localStorage.removeItem('analytics_token'); router.push('/login') }
</script>

<template>
  <RouterView v-if="isLogin" />
  <div v-else class="app-shell">
    <aside class="sidebar">
      <div class="brand"><span class="brand-mark"><GitCommitHorizontal :size="22" /></span><span>GitLab<br /><b>贡献分析</b></span></div>
      <nav aria-label="主导航">
        <RouterLink v-for="link in links" :key="link.to" :to="link.to" class="nav-link">
          <component :is="link.icon" :size="18" /><span>{{ link.label }}</span>
        </RouterLink>
      </nav>
      <div class="sidebar-bottom"><span class="status-dot"></span><span>本地数据仓库</span></div>
    </aside>
    <main class="main-content">
      <header class="topbar"><div><p class="eyebrow">GITLAB ANALYTICS</p><h1>{{ route.meta.title }}</h1></div><button class="icon-button logout" title="退出登录" @click="logout"><LogOut :size="18" /><span>退出</span></button></header>
      <RouterView />
    </main>
  </div>
</template>
