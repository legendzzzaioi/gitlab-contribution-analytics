<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { GitCommitHorizontal, LockKeyhole, UserRound, ArrowRight } from 'lucide-vue-next'

const router = useRouter()
const username = ref('admin')
const password = ref('change-me-now')
const error = ref('')
const submitting = ref(false)

// 登录
async function login() {
  error.value = ''
  submitting.value = true
  try {
    const response = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: username.value,
        password: password.value,
      }),
    })
    
    // 1. 先判断响应状态码是否正常
    if (!response.ok) {
      // 如果返回了 500 等错误，且内容可能是非 JSON（如 HTML 报错页）
      if (response.status === 500 || response.status === 502 || response.status === 504) {
        throw new Error('无法连接到服务器，请检查后端服务是否正常')
      }
      
      // 尝试解析后端返回的具体错误信息
      let errorMsg = '登录失败'
      try {
        const data = await response.json()
        if (data && data.message) errorMsg = data.message
      } catch (e) {
        // 如果解析 JSON 失败，说明后端返回了空或者 HTML
      }
      throw new Error(errorMsg)
    }

    // 2. 确认是 200 OK 之后，再安全解析 JSON
    const data = await response.json()
    localStorage.setItem('analytics_token', data.token)
    router.push('/')
    
  } catch (err) {
    if (err.message.includes('Unexpected end of JSON input') || err.message.includes('Failed to fetch')) {
      error.value = '网络请求失败，请检查接口地址或后端服务状态'
    } else {
      error.value = err.message
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="login-page">
    <section class="login-panel">
      <div class="login-brand">
        <span class="brand-mark"><GitCommitHorizontal :size="26" /></span>
        <div>
          <p>GITLAB ANALYTICS</p>
          <h1>贡献分析中心</h1>
        </div>
      </div>
      <p class="login-copy">将 GitLab 的提交活动沉淀为可追溯、可比较的团队数据。</p>
      <form @submit.prevent="login">
        <label>
          管理员账号
          <div class="input-icon">
            <UserRound :size="17" />
            <input v-model="username" autocomplete="username" required />
          </div>
        </label>
        <label>
          登录密码
          <div class="input-icon">
            <LockKeyhole :size="17" />
            <input v-model="password" type="password" autocomplete="current-password" required />
          </div>
        </label>
        <p v-if="error" class="form-error">{{ error }}</p>
        <button class="button primary login-submit" :disabled="submitting">
          {{ submitting ? '正在验证…' : '登录并查看数据' }}
          <ArrowRight :size="17" />
        </button>
      </form>
      <p class="login-help">
        首次运行默认账号：admin / change-me-now<br />
        请在生产环境通过环境变量修改。
      </p>
    </section>

    <aside class="login-aside">
      <div>
        <p class="eyebrow">LOCAL DATA WAREHOUSE</p>
        <h2>采集一次，<br />随时分析。</h2>
        <p>定时增量同步 GitLab API 到本地 MySQL。所有图表和筛选均来自本地数据，不给 GitLab 生产环境增加查询压力。</p>
      </div>
      <div class="login-grid">
        <span>提交</span>
        <b>+ / −</b>
        <span>贡献者</span>
        <b>趋势</b>
      </div>
    </aside>
  </main>
</template>