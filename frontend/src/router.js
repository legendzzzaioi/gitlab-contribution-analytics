import { createRouter, createWebHistory } from 'vue-router'
const DashboardView = () => import('./views/DashboardView.vue')
const ContributionsView = () => import('./views/ContributionsView.vue')
const CommitsView = () => import('./views/CommitsView.vue')
const SyncView = () => import('./views/SyncView.vue')
const SettingsView = () => import('./views/SettingsView.vue')
const LoginView = () => import('./views/LoginView.vue')

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: LoginView, meta: { public: true } },
    { path: '/', component: DashboardView, meta: { title: '总览' } },
    { path: '/contributions', component: ContributionsView, meta: { title: '贡献者' } },
    { path: '/commits', component: CommitsView, meta: { title: '提交明细' } },
    { path: '/sync', component: SyncView, meta: { title: '同步中心' } },
    { path: '/settings', component: SettingsView, meta: { title: '数据源配置' } }
  ]
})
router.beforeEach((to) => {
  if (!to.meta.public && !localStorage.getItem('analytics_token')) return '/login'
  if (to.path === '/login' && localStorage.getItem('analytics_token')) return '/'
})
export default router
