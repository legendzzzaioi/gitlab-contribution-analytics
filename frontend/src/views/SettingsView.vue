<script setup>
import { onMounted, reactive, ref } from 'vue'
import { Check, KeyRound, Link2, Plus, Save, TestTube2, Trash2, UsersRound } from 'lucide-vue-next'
import { api } from '../api'

const form = reactive({
  gitlabUrl: '',
  gitlabToken: '',
  rootGroupIds: '',
  syncCron: '0 */12 * * *',
})

const tokenConfigured = ref(false)
const maskedToken = ref('')
const loading = ref(false)
const testing = ref(false)
const message = ref('')
const error = ref('')
const users = ref([])
const aliases = ref({})
const aliasesLoading = ref(false)
const aliasInput = reactive({})
// 加载设置
async function loadSettings() {
  try {
    const data = await api('/settings')
    form.gitlabUrl = data.gitlabUrl || ''
    form.rootGroupIds = data.rootGroupIds || ''
    form.syncCron = data.syncCron || '0 */12 * * *'
    tokenConfigured.value = data.tokenConfigured
    maskedToken.value = data.maskedToken || ''
  } catch (err) {
    error.value = err.message
  }
}

// 加载用户数据
async function loadUsers() {
  aliasesLoading.value = true
  try {
    const data = await api('/users')
    users.value = data.items || []
    aliases.value = data.aliases || {}
  } catch (err) {
    error.value = err.message
  } finally {
    aliasesLoading.value = false
  }
}

// 测试配置
async function test() {
  testing.value = true
  message.value = ''
  error.value = ''
  try {
    const result = await api('/settings/test', {
      method: 'POST',
      body: JSON.stringify(form),
    })
    message.value = result.message
  } catch (err) {
    error.value = err.message
  } finally {
    testing.value = false
  }
}

// 保存配置
async function save() {
  loading.value = true
  message.value = ''
  error.value = ''
  try {
    const result = await api('/settings', {
      method: 'PUT',
      body: JSON.stringify(form),
    })
    form.gitlabToken = ''
    tokenConfigured.value = result.tokenConfigured
    maskedToken.value = result.maskedToken
    message.value = '配置已保存，同步计划已更新。'
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

// 添加邮箱别名
async function addAlias(user) {
  const email = aliasInput[user.id]
  if (!email) return
  error.value = ''
  try {
    await api(`/users/${user.id}/aliases`, {
      method: 'POST',
      body: JSON.stringify({ email }),
    })
    aliasInput[user.id] = ''
    await loadUsers()
    message.value = '邮箱别名已保存，历史提交已重新归属。'
  } catch (err) {
    error.value = err.message
  }
}

// 删除邮箱别名
async function removeAlias(alias) {
  try {
    await api(`/aliases/${alias.id}`, { method: 'DELETE' })
    await loadUsers()
  } catch (err) {
    error.value = err.message
  }
}

// 组件挂载时加载设置和用户数据
onMounted(async () => {
  await Promise.all([loadSettings(), loadUsers()])
})
</script>

<template>
  <div class="settings-grid">
    <section class="panel settings-panel">
      <div class="panel-head">
        <div>
          <h2>GitLab 连接</h2>
          <p>使用只读服务账号，令牌不会回显。</p>
        </div>
        <Link2 :size="19" />
      </div>
      <form class="form-grid" @submit.prevent="save">
        <label>
          GitLab 地址
          <input v-model="form.gitlabUrl" type="url" placeholder="https://gitlab.company.com" required />
        </label>
        <label>
          访问令牌
          <span class="label-note" v-if="tokenConfigured">已保存：{{ maskedToken }}；留空则不修改</span>
          <div class="input-icon">
            <KeyRound :size="17" />
            <input v-model="form.gitlabToken" type="password" placeholder="glpat-… 或项目访问令牌" autocomplete="new-password" />
          </div>
        </label>
        <label>
          根项目组 ID
          <span class="label-note">可选。留空同步 Token 可见的全部顶级项目组及其子组；填写多个 ID 时用逗号分隔。</span>
          <input v-model="form.rootGroupIds" placeholder="留空同步全部，或例如：123, 456" />
        </label>
        <label>
          同步计划（Cron）
          <span class="label-note">标准 5 段格式，如每 12 小时：0 */12 * * *</span>
          <input v-model="form.syncCron" placeholder="0 */12 * * *" required />
        </label>
        <div class="form-actions">
          <button class="button secondary" type="button" :disabled="testing" @click="test">
            <TestTube2 :size="16" />
            {{ testing ? '正在连接…' : '测试连接' }}
          </button>
          <button class="button primary" :disabled="loading">
            <Save :size="16" />
            {{ loading ? '正在保存…' : '保存配置' }}
          </button>
        </div>
      </form>
      <p v-if="message" class="success-banner">
        <Check :size="16" />
        {{ message }}
      </p>
      <p v-if="error" class="error-banner">{{ error }}</p>
    </section>

    <aside class="settings-help panel">
      <h3>最小权限建议</h3>
      <ol>
        <li>新建专用服务账号。</li>
        <li>将其赋予目标顶级项目组 Reporter 或更高读取权限。</li>
        <li>创建仅含 <code>read_api</code> 的 Access Token。</li>
        <li>在本页填入项目组数字 ID，不要填路径。</li>
      </ol>
      <p>首次执行“全量同步”后，系统会基于上次成功时间定时增量更新。</p>
    </aside>
  </div>

  <section class="panel alias-panel">
    <div class="panel-head">
      <div>
        <h2>贡献者与邮箱别名</h2>
        <p>同一人使用多个 commit email 时，将邮箱别名添加到主贡献者，历史数据会立即重新归并。</p>
      </div>
      <UsersRound :size="19" />
    </div>
    <div class="table-scroll">
      <table>
        <thead>
          <tr>
            <th>贡献者</th>
            <th>主邮箱</th>
            <th>已映射邮箱别名</th>
            <th>新增别名</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users" :key="user.id">
            <td>
              <div class="person">
                <div class="avatar">{{ user.name?.slice(0, 1) }}</div>
                <div>
                  <b>{{ user.name }}</b>
                  <small>
                    {{ user.username || '—' }}
                    <span v-if="user.isBot" class="tag">机器人</span>
                  </small>
                </div>
              </div>
            </td>
            <td>{{ user.email }}</td>
            <td>
              <div class="alias-list">
                <span v-for="alias in aliases[user.id] || []" :key="alias.id" class="alias-chip">
                  {{ alias.email }}
                  <button title="删除别名" @click="removeAlias(alias)">
                    <Trash2 :size="12" />
                  </button>
                </span>
                <span v-if="!(aliases[user.id] || []).length" class="muted">无</span>
              </div>
            </td>
            <td>
              <div class="add-alias">
                <input v-model="aliasInput[user.id]" type="email" placeholder="name@company.com" @keyup.enter="addAlias(user)" />
                <button class="button ghost small" :disabled="aliasesLoading" @click="addAlias(user)">
                  <Plus :size="15" />
                  添加
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="!users.length">
            <td colspan="4" class="empty-state">同步 GitLab 提交后，贡献者将自动出现在这里。</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>