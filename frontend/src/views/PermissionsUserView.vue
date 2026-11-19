
<script setup>
import { ref, computed, onMounted } from 'vue'
import { api, queryString } from '../api'
import {
  Search,
  UserRound,
  ChevronDown,
  FolderTree,
  FolderGit2,
  ExternalLink,
  RefreshCw,
  HardDrive,
  CalendarDays,
  GitCommitHorizontal
} from 'lucide-vue-next'

const users = ref([])
const search = ref('')
const loading = ref(false)

const expandedUserId = ref(null)
const details = ref({})
const loadingUserId = ref(null)

// 加载用户列表
const loadUsers = async () => {
  loading.value = true
  expandedUserId.value = null
  details.value = {}
  currentPage.value = 1

  try {
    const res = await api(`/permissions/users${queryString({
      search: search.value.trim()
    })}`)

    users.value = res.users || []
  } catch (error) {
    console.error('加载用户列表失败:', error)
    users.value = []
  } finally {
    loading.value = false
  }
}

// 切换用户详情展开/收缩
const toggleUser = async (user) => {
  if (expandedUserId.value === user.userId) {
    expandedUserId.value = null
    return
  }

  expandedUserId.value = user.userId

  // 已经加载过的用户直接展开，不重复请求
  if (details.value[user.userId]) return

  loadingUserId.value = user.userId

  try {
    const res = await api(`/permissions/users/${user.userId}`)

    details.value[user.userId] = {
      groups: res.groups || [],
      projects: res.projects || []
    }
  } catch (error) {
    console.error('加载用户权限详情失败:', error)
  } finally {
    loadingUserId.value = null
  }
}

// 获取权限级别名称
const levelName = (level) => {
  if (level >= 50) return 'Owner'
  if (level >= 40) return 'Maintainer'
  if (level >= 30) return 'Developer'
  if (level >= 20) return 'Reporter'
  return 'Guest'
}

// 获取权限级别样式类
const levelClass = (level) => {
  if (level >= 50) return 'owner'
  if (level >= 40) return 'maintainer'
  if (level >= 30) return 'developer'
  if (level >= 20) return 'reporter'
  return 'guest'
}

// 格式化创建时间
const formatDate = (value) => {
  if (!value) return '-'

  const date = new Date(value)

  if (Number.isNaN(date.getTime())) {
    return '-'
  }

  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  })
}

// 格式化仓库大小，单位 Bytes
const formatSize = (bytes) => {
  if (bytes === null || bytes === undefined || bytes < 0) {
    return '-'
  }

  if (bytes === 0) {
    return '0 B'
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1
  )

  return `${(bytes / Math.pow(1024, index)).toFixed(index === 0 ? 0 : 2)} ${units[index]}`
}

const searchUsers = () => loadUsers()

// 分页
const currentPage = ref(1)
const pageSize = ref(10)

const totalPages = computed(() => {
  return Math.max(1, Math.ceil(users.value.length / pageSize.value))
})

const paginatedUsers = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return users.value.slice(start, start + pageSize.value)
})

const pageNumbers = computed(() => {
  const total = totalPages.value
  const current = currentPage.value
  const pages = []

  let start = Math.max(1, current - 2)
  let end = Math.min(total, current + 2)

  if (current <= 3) {
    end = Math.min(total, 5)
  }

  if (current >= total - 2) {
    start = Math.max(1, total - 4)
  }

  for (let i = start; i <= end; i++) {
    pages.push(i)
  }

  return pages
})

const changePage = (page) => {
  if (page < 1 || page > totalPages.value) return

  currentPage.value = page
  expandedUserId.value = null
}

const changePageSize = () => {
  currentPage.value = 1
  expandedUserId.value = null
}

onMounted(loadUsers)
</script>


<template>
  <section class="panel user-permission-page">
    <!-- 页面标题及搜索 -->
    <div class="page-toolbar">
      <div class="page-title">
        <div class="title-icon">
          <UserRound :size="19" />
        </div>

        <div>
          <h2>用户权限</h2>
          <p>查看用户拥有的群组和项目访问权限</p>
        </div>
      </div>

      <div class="search-area">
        <div class="search-box">
          <Search :size="16" />

          <input
            v-model="search"
            placeholder="搜索姓名或用户名..."
            @keyup.enter="searchUsers"
          />

          <button
            v-if="search"
            class="clear-btn"
            @click="search = ''; searchUsers()"
          >
            ×
          </button>
        </div>

        <button
          class="search-btn"
          :disabled="loading"
          @click="searchUsers"
        >
          查询
        </button>

        <button
          class="refresh-btn"
          :disabled="loading"
          title="刷新用户列表"
          @click="loadUsers"
        >
          <RefreshCw :size="16" />
        </button>
      </div>
    </div>

    <!-- 用户数量 -->
    <div class="list-summary">
      <span>用户列表</span>
      <span class="count">{{ users.length }}</span>
    </div>

    <!-- 用户列表 -->
    <div class="user-list">
      <div v-if="loading" class="empty-state">
        <RefreshCw :size="18" class="spin" />
        正在加载用户...
      </div>

      <div v-else-if="!users.length" class="empty-state">
        <UserRound :size="28" />
        <span>没有找到用户</span>
      </div>

      <div
        v-for="user in paginatedUsers"
        :key="user.userId"
        class="user-item"
      >
        <!-- 用户行 -->
        <div
          class="user-row"
          :class="{ active: expandedUserId === user.userId }"
          @click="toggleUser(user)"
        >
          <div class="user-main">
            <div class="user-avatar">
              {{ (user.userName || user.username || '?').slice(0, 1).toUpperCase() }}
            </div>

            <div class="user-info">
              <div class="user-name">
                {{ user.userName || '-' }}
              </div>

              <div class="username">
                @{{ user.username || '-' }}
              </div>
            </div>
          </div>

          <div class="user-stats">
            <span class="stat-item">
              <FolderTree :size="14" />
              {{ user.groupCount }} 个群组
            </span>

            <span class="stat-item">
              <FolderGit2 :size="14" />
              {{ user.projectCount }} 个项目
            </span>
          </div>

          <div class="expand-icon">
            <ChevronDown
              :size="18"
              :class="{ rotated: expandedUserId === user.userId }"
            />
          </div>
        </div>

        <!-- 用户权限详情 -->
        <div
          v-if="expandedUserId === user.userId"
          class="user-detail"
        >
          <div
            v-if="loadingUserId === user.userId"
            class="detail-loading"
          >
            <RefreshCw :size="17" class="spin" />
            正在加载权限详情...
          </div>

          <template v-else-if="details[user.userId]">
            <!-- 群组权限 -->
            <div class="resource-section">
              <div class="section-heading">
                <div class="section-title">
                  <FolderTree :size="16" />
                  群组

                  <span class="section-count">
                    {{ details[user.userId].groups.length }}
                  </span>
                </div>
              </div>

              <div
                v-if="details[user.userId].groups.length"
                class="resource-list"
              >
                <div
                  v-for="item in details[user.userId].groups"
                  :key="item.targetGitlabId"
                  class="resource-row"
                >
                  <!-- 群组信息 -->
                  <div class="resource-main">
                    <!-- 第一行：名称 + 描述 -->
                    <div class="resource-title-line">
                      <a
                        v-if="item.targetUrl"
                        :href="item.targetUrl"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="resource-name"
                      >
                        {{ item.targetName || item.targetPath }}
                        <ExternalLink :size="12" />
                      </a>

                      <span
                        v-else
                        class="resource-name"
                      >
                        {{ item.targetName || `群组 ${item.targetGitlabId}` }}
                      </span>

                      <span
                        v-if="item.description"
                        class="inline-description"
                        :title="item.description"
                      >
                        {{ item.description }}
                      </span>
                    </div>

                    <!-- 第二行：路径 + 创建时间 -->
                    <div class="resource-meta">
                      <span class="resource-path">
                        {{ item.targetPath || '-' }}
                      </span>

                      <span class="meta-item">
                        <CalendarDays :size="13" />
                        创建于 {{ formatDate(item.creationDate) }}
                      </span>
                    </div>
                  </div>

                  <!-- 权限信息 -->
                  <div class="resource-right">
                    <span
                      class="level-tag"
                      :class="levelClass(item.accessLevel)"
                    >
                      {{ levelName(item.accessLevel) }}
                    </span>

                    <span
                      v-if="item.inherited"
                      class="inherit-tag"
                      :title="item.sourceTargetName"
                    >
                      继承自 {{ item.sourceTargetName || '未知来源' }}
                    </span>

                    <span
                      v-else
                      class="direct-tag"
                    >
                      直接授权
                    </span>
                  </div>
                </div>
              </div>

              <div
                v-else
                class="resource-empty"
              >
                暂无群组权限
              </div>
            </div>

            <!-- 项目权限 -->
            <div class="resource-section project-section">
              <div class="section-heading">
                <div class="section-title">
                  <FolderGit2 :size="16" />
                  加入的项目

                  <span class="section-count">
                    {{ details[user.userId].projects.length }}
                  </span>
                </div>
              </div>

              <div
                v-if="details[user.userId].projects.length"
                class="resource-list"
              >
                <div
                  v-for="item in details[user.userId].projects"
                  :key="item.targetGitlabId"
                  class="resource-row project-resource-row"
                >
                  <!-- 项目信息 -->
                  <div class="resource-main">
                    <!-- 第一行：名称 + 描述 -->
                    <div class="resource-title-line">
                      <a
                        v-if="item.targetUrl"
                        :href="item.targetUrl"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="resource-name"
                      >
                        {{ item.targetName || item.targetPath }}
                        <ExternalLink :size="12" />
                      </a>

                      <span
                        v-else
                        class="resource-name"
                      >
                        {{ item.targetName || `项目 ${item.targetGitlabId}` }}
                      </span>

                      <span
                        v-if="item.description"
                        class="inline-description"
                        :title="item.description"
                      >
                        {{ item.description }}
                      </span>
                    </div>

                    <!-- 第二行：路径 + 创建时间 + 提交次数 + 存储大小 -->
                    <div class="resource-meta">
                      <span class="resource-path">
                        {{ item.targetPath || '-' }}
                      </span>

                      <span class="meta-item">
                        <CalendarDays :size="13" />
                        创建于 {{ formatDate(item.creationDate) }}
                      </span>

                      <span class="meta-item">
                        <GitCommitHorizontal :size="13" />
                        {{ item.commitCount ?? 0 }} 次提交
                      </span>

                      <span class="meta-item">
                        <HardDrive :size="13" />
                        {{ formatSize(item.storageSize) }}
                      </span>
                    </div>
                  </div>

                  <!-- 权限信息 -->
                  <div class="resource-right">
                    <span
                      class="level-tag"
                      :class="levelClass(item.accessLevel)"
                    >
                      {{ levelName(item.accessLevel) }}
                    </span>

                    <span
                      v-if="item.inherited"
                      class="inherit-tag"
                      :title="item.sourceTargetName"
                    >
                      继承自 {{ item.sourceTargetName || '未知来源' }}
                    </span>

                    <span
                      v-else
                      class="direct-tag"
                    >
                      直接授权
                    </span>
                  </div>
                </div>
              </div>

              <div
                v-else
                class="resource-empty"
              >
                暂无项目权限
              </div>
            </div>
          </template>
        </div>
      </div>
        <!-- 分页 -->
        <div v-if="users.length" class="pagination">
        <div class="pagination-info">
            <span>共 {{ users.length }} 条</span>

            <select
            v-model.number="pageSize"
            @change="changePageSize"
            >
            <option :value="10">10 条/页</option>
            <option :value="20">20 条/页</option>
            <option :value="50">50 条/页</option>
            <option :value="100">100 条/页</option>
            </select>
        </div>

        <div class="pagination-controls">
            <button
            class="page-btn"
            :disabled="currentPage <= 1"
            @click="changePage(currentPage - 1)"
            >
            上一页
            </button>

            <button
            v-for="page in pageNumbers"
            :key="page"
            class="page-btn"
            :class="{ active: currentPage === page }"
            @click="changePage(page)"
            >
            {{ page }}
            </button>

            <button
            class="page-btn"
            :disabled="currentPage >= totalPages"
            @click="changePage(currentPage + 1)"
            >
            下一页
            </button>

            <span class="page-total">
            {{ currentPage }} / {{ totalPages }}
            </span>
        </div>
        </div>
    </div>
  </section>
</template>


<style scoped>
.user-permission-page {
  overflow: hidden;
}

/* 页面标题及搜索 */
.page-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 20px 22px;
  border-bottom: 1px solid #edf0f5;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 11px;
}

.title-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: 9px;
  background: #e6f6f3;
  color: #178a70;
}

.page-title h2 {
  margin: 0;
  color: #15213b;
  font-size: 16px;
  font-weight: 600;
}

.page-title p {
  margin: 5px 0 0;
  color: #94a3b8;
  font-size: 12px;
}

.search-area {
  display: flex;
  align-items: center;
  gap: 8px;
}

.search-box {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 250px;
  height: 36px;
  padding: 0 10px;
  border: 1px solid #dbe2ed;
  border-radius: 7px;
  color: #94a3b8;
}

.search-box:focus-within {
  border-color: #178a70;
}

.search-box input {
  width: 100%;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  font-size: 13px;
}

.clear-btn {
  border: none;
  background: transparent;
  color: #94a3b8;
  cursor: pointer;
  font-size: 18px;
}

.search-btn,
.refresh-btn {
  height: 36px;
  border: 1px solid #dbe2ed;
  border-radius: 7px;
  background: #fff;
  color: #475569;
  cursor: pointer;
}

.search-btn {
  padding: 0 16px;
  background: #178a70;
  border-color: #178a70;
  color: #fff;
}

.refresh-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
}

.search-btn:disabled,
.refresh-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* 用户列表 */
.list-summary {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 14px 22px;
  color: #475569;
  font-size: 13px;
  font-weight: 600;
}

.count,
.section-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 21px;
  height: 21px;
  padding: 0 6px;
  border-radius: 10px;
  background: #f1f5f9;
  color: #64748b;
  font-size: 11px;
}

.user-list {
  padding: 0 14px 14px;
}

.user-item {
  margin-bottom: 8px;
  overflow: hidden;
  border: 1px solid #e8edf3;
  border-radius: 9px;
  background: #fff;
}

.user-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 66px;
  padding: 12px 16px;
  cursor: pointer;
  transition: background 0.15s;
}

.user-row:hover,
.user-row.active {
  background: #f8fafc;
}

.user-main {
  display: flex;
  align-items: center;
  gap: 11px;
  min-width: 200px;
}

.user-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  border-radius: 50%;
  background: #e6f6f3;
  color: #178a70;
  font-size: 14px;
  font-weight: 600;
}

.user-name {
  color: #1e293b;
  font-size: 13px;
  font-weight: 600;
}

.username {
  margin-top: 4px;
  color: #94a3b8;
  font-size: 11px;
}

.user-stats {
  display: flex;
  align-items: center;
  gap: 18px;
  margin-left: auto;
}

.stat-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: #64748b;
  font-size: 12px;
  white-space: nowrap;
}

.expand-icon {
  display: flex;
  align-items: center;
  color: #94a3b8;
}

.expand-icon svg {
  transition: transform 0.2s;
}

.expand-icon svg.rotated {
  transform: rotate(180deg);
}

/* 用户详情 */
.user-detail {
  padding: 4px 16px 16px 62px;
  border-top: 1px solid #edf0f5;
  background: #fafbfc;
}

.resource-section {
  margin-top: 14px;
  overflow: hidden;
  border: 1px solid #e8edf3;
  border-radius: 8px;
  background: #fff;
}

.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 11px 14px;
  background: #f1f3f6;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.section-title svg {
  color: #64748b;
}


/* 资源列表 */
.resource-list {
  padding: 0 10px;
}

.resource-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  min-height: 68px;
  padding: 12px 10px;
  border-bottom: 1px solid #edf0f5;
}

.resource-row:last-child {
  border-bottom: none;
}

/* 左侧资源信息 */
.resource-main {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

/* 第一行：名称 + 描述 */
.resource-title-line {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  overflow: hidden;
}

.resource-name {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  flex-shrink: 0;
  max-width: 45%;
  color: #2864b9;
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

a.resource-name:hover {
  text-decoration: underline;
}

.inline-description {
  min-width: 0;
  overflow: hidden;
  color: #64748b;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 第二行：路径 + 其他信息 */
.resource-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 14px;
  min-width: 0;
}

.resource-path {
  min-width: 0;
  color: #94a3b8;
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: #94a3b8;
  font-size: 11px;
  white-space: nowrap;
  flex-shrink: 0;
}

.meta-item svg {
  color: #94a3b8;
}

/* 右侧权限信息 */
.resource-right {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  flex-shrink: 0;
}

.level-tag {
  min-width: 88px;
  text-align: center;
  font-size: 12px;
  font-weight: 500;
}

.level-tag.owner {
  color: #d65b21;
}

.level-tag.maintainer {
  color: #8053c7;
}

.level-tag.developer {
  color: #178a70;
}

.level-tag.reporter {
  color: #3569ed;
}

.level-tag.guest {
  color: #64748b;
}

.inherit-tag,
.direct-tag {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
}

.inherit-tag {
  color: #b7791f;
}

.direct-tag {
  color: #178a70;
}

/* 移动端 */
@media (max-width: 768px) {
  .resource-row {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
  }

  .resource-title-line {
    align-items: flex-start;
    flex-direction: column;
    gap: 5px;
  }

  .resource-name {
    max-width: 100%;
  }

  .inline-description {
    max-width: 100%;
    white-space: normal;
  }

  .resource-meta {
    gap: 8px;
  }

  .resource-right {
    width: 100%;
    justify-content: flex-start;
    flex-wrap: wrap;
  }
}

/* 资源元数据 */
.resource-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: #94a3b8;
  font-size: 11px;
  white-space: nowrap;
}

.meta-item svg {
  color: #94a3b8;
}

/* 资源描述 */
.resource-description {
  display: block;
  color: #64748b;
  font-size: 11px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.project-description {
  padding: 7px 9px;
  border-radius: 5px;
  background: #f8fafc;
}

/* 权限标签 */
.resource-right {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  flex-shrink: 0;
}

.level-tag {
  min-width: 88px;
  text-align: center;
  font-size: 12px;
  font-weight: 500;
}

.level-tag.owner {
  color: #d65b21;
}

.level-tag.maintainer {
  color: #8053c7;
}

.level-tag.developer {
  color: #178a70;
}

.level-tag.reporter {
  color: #3569ed;
}

.level-tag.guest {
  color: #64748b;
}

.inherit-tag,
.direct-tag {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
}

.inherit-tag {
  color: #b7791f;
}

.direct-tag {
  color: #178a70;
}

/* 空状态及加载状态 */
.resource-empty {
  padding: 20px;
  color: #a0aab8;
  text-align: center;
  font-size: 12px;
}

.empty-state,
.detail-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  padding: 35px;
  color: #94a3b8;
  font-size: 13px;
}

.empty-state {
  flex-direction: column;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

/* 移动端 */
@media (max-width: 768px) {
  .page-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .search-box {
    flex: 1;
    width: auto;
  }

  .user-row {
    flex-wrap: wrap;
  }

  .user-stats {
    margin-left: 47px;
    order: 3;
    width: calc(100% - 47px);
  }

  .expand-icon {
    margin-left: auto;
  }

  .user-detail {
    padding: 4px 8px 12px;
  }

  .resource-row {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
  }

  .resource-right {
    width: 100%;
    justify-content: flex-start;
    flex-wrap: wrap;
  }

  .resource-meta {
    gap: 8px;
  }
}

/* 分页 */
.summary-text {
  color: #94a3b8;
  font-size: 12px;
  font-weight: 400;
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 4px 4px;
}

.pagination-info {
  display: flex;
  align-items: center;
  gap: 12px;
  color: #94a3b8;
  font-size: 12px;
}

.pagination-info select {
  height: 30px;
  padding: 0 8px;
  border: 1px solid #dbe2ed;
  border-radius: 6px;
  outline: none;
  background: #fff;
  color: #475569;
  font-size: 12px;
  cursor: pointer;
}

.pagination-controls {
  display: flex;
  align-items: center;
  gap: 5px;
}

.page-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 30px;
  height: 30px;
  padding: 0 9px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  background: #fff;
  color: #64748b;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
}

.page-btn:hover:not(:disabled) {
  border-color: #178a70;
  color: #178a70;
}

.page-btn.active {
  border-color: #178a70;
  background: #178a70;
  color: #fff;
}

.page-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.page-total {
  margin-left: 6px;
  color: #94a3b8;
  font-size: 12px;
  white-space: nowrap;
}

@media (max-width: 768px) {
  .pagination {
    align-items: flex-start;
    flex-direction: column;
  }

  .pagination-controls {
    width: 100%;
    flex-wrap: wrap;
  }
}
</style>
