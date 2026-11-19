
<script setup>
import dayjs from 'dayjs'
import { reactive } from 'vue'
import GroupTreeNode from '../components/GroupTreeNode.vue'
import { ref, computed, onMounted, defineComponent, h } from 'vue'
import { api } from '../api'
import {
  Folder,
  FolderOpen,
  FileCode2,
  Shield,
  ChevronRight,
  ChevronDown,
  RefreshCw,
  Users,
  ChevronUp,
  GitCommit, 
  HardDrive, 
  CalendarDays
} from 'lucide-vue-next'

const groups = ref([])
const projects = ref([])
const members = ref([])

const activeGroupId = ref(null)
const loading = ref(false)
const errorMessage = ref('')

// 构建 Group 树
const groupTree = computed(() => {
  const nodeMap = new Map()
  const roots = []

  // 创建节点
  for (const group of groups.value) {
    const id = Number(group.gitlabId)

    nodeMap.set(id, {
      ...group,
      children: [],
    })
  }

  // 建立父子关系
  for (const node of nodeMap.values()) {
    const parentId = Number(node.parentGitlabId)
    const parent = nodeMap.get(parentId)

    if (parent && parent !== node) {
      parent.children.push(node)
    } else {
      // 父节点不在当前数据集中的，作为当前树的根节点
      roots.push(node)
    }
  }

  // 按名称排序
  function sortNodes(nodes) {
    nodes.sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))

    for (const node of nodes) {
      sortNodes(node.children)
    }
  }

  sortNodes(roots)

  return roots
})

// 加载数据
const load = async () => {
  loading.value = true
  errorMessage.value = ''

  try {
    const res = await api('/permissions/tree')

    groups.value = res.groups || []
    projects.value = res.projects || []
    members.value = res.members || []

    // 默认选择第一个根 Group
    if (groups.value.length > 0) {
      const currentExists = groups.value.some(
        g => Number(g.gitlabId) === Number(activeGroupId.value)
      )

      if (!currentExists) {
        activeGroupId.value =
          groupTree.value[0]?.gitlabId ?? groups.value[0].gitlabId
      }
    } else {
      activeGroupId.value = null
    }

    // 默认展开所有根节点
    const next = new Set(expandedKeys.value)

    groupTree.value.forEach(node => {
      next.add(node.gitlabId)
    })

    expandedKeys.value = next
  } catch (error) {
    console.error('加载权限数据失败:', error)
    errorMessage.value = error.message || '加载权限数据失败'
  } finally {
    loading.value = false
  }
}

// 当前选中的 Group
const activeGroup = computed(() => {
  return groups.value.find(
    g => Number(g.gitlabId) === Number(activeGroupId.value)
  )
})

// 当前 Group 直接关联的项目
const activeProjects = computed(() => {
  return projects.value.filter(
    p => Number(p.group?.gitlabId) === Number(activeGroupId.value)
  )
})

// 当前 Group 的有效成员
const activeGroupMembers = computed(() => {
  return members.value.filter(
    m =>
      m.targetType === 'group' &&
      Number(m.targetGitlabId) === Number(activeGroupId.value)
  )
})

// 项目有效成员
const getProjectMembers = projectId => {
  return members.value.filter(
    m =>
      m.targetType === 'project' &&
      Number(m.targetGitlabId) === Number(projectId)
  )
}

// 直接成员
const directMembers = list => {
  return list.filter(m => !m.inherited)
}

// 继承成员
const inheritedMembers = list => {
  return list.filter(m => m.inherited)
}

// 权限名称
const levelName = level => {
  if (level >= 50) return 'Owner'
  if (level >= 40) return 'Maintainer'
  if (level >= 30) return 'Developer'
  if (level >= 20) return 'Reporter'
  if (level >= 10) return 'Guest'
  return 'Unknown'
}

// 权限颜色
const levelClass = level => {
  if (level >= 50) return 'owner'
  if (level >= 40) return 'maintainer'
  if (level >= 30) return 'developer'
  if (level >= 20) return 'reporter'
  return 'guest'
}

// 获取授权来源
const sourceName = member => {
  if (!member.inherited) {
    return '当前对象直接授权'
  }

  const sourceId = Number(member.sourceTargetGitlabId)

  if (!sourceId) {
    return '继承权限（来源未识别）'
  }

  if (member.sourceTargetType === 'group') {
    const source = groups.value.find(
      group => Number(group.gitlabId) === sourceId
    )

    return source
      ? `继承自 ${source.fullPath}`
      : `继承自 Group #${sourceId}`
  }

  if (member.sourceTargetType === 'project') {
    const source = projects.value.find(
      project => Number(project.gitlabId) === sourceId
    )

    return source
      ? `继承自 ${source.pathWithNS}`
      : `继承自 Project #${sourceId}`
  }

  return '继承权限（来源未识别）'
}

// 选择 Group
function selectGroup(group) {
  activeGroupId.value = Number(group.gitlabId)
}

const expandedKeys = reactive(new Set())

function toggleGroup(groupId) {
  if (expandedKeys.has(groupId)) {
    expandedKeys.delete(groupId)
  } else {
    expandedKeys.add(groupId)
  }
}

function expandAllGroups(nodes) {
  for (const node of nodes) {
    if (node.children.length > 0) {
      expandedKeys.add(Number(node.gitlabId))
      expandAllGroups(node.children)
    }
  }
}

function collapseAllGroups() {
  expandedKeys.clear()
}

const groupMap = computed(() => {
  const map = new Map()

  for (const group of groups.value) {
    map.set(Number(group.gitlabId), group)
  }

  return map
})

onMounted(load)
</script>


<template>
  <div>
    <div v-if="errorMessage" class="error-panel">
      {{ errorMessage }}
    </div>

    <div class="perm-layout">
      <!-- 左侧 Group 树 -->
      <aside class="panel group-panel">
        <div class="group-panel-head">
          <div class="group-title">
            <div>
              <h2>群组层级</h2>
              <span class="group-count">{{ groups.length }} 个群组</span>
            </div>
          </div>

          <div class="tree-actions">
            <!-- 刷新数据 -->
            <button
              class="tree-action-btn"
              @click="load"
              :disabled="loading"
              title="刷新数据"
            >
              <RefreshCw
                :size="14"
                :class="{ spinning: loading }"
              />
            </button>

            <!-- 展开全部 -->
            <button
              class="tree-action-btn"
              @click="expandAllGroups(groupTree)"
              title="展开所有层级"
            >
              <ChevronDown :size="14" />
            </button>

            <!-- 收起全部 -->
            <button
              class="tree-action-btn"
              @click="collapseAllGroups"
              title="收起所有层级"
            >
              <ChevronUp :size="14" />
            </button>
          </div>
        </div>

        <div class="group-tree">
          <GroupTreeNode
            v-for="group in groupTree"
            :key="group.gitlabId"
            :node="group"
            :active-group-id="activeGroupId"
            :expanded-keys="expandedKeys"
            @select="selectGroup"
            @toggle="toggleGroup"
          />
        </div>
      </aside>

      <!-- 右侧权限详情 -->
      <main class="main-panel" v-if="activeGroup">
        <!-- Group 基础信息 -->
        <div class="panel detail-panel">
          <div class="panel-head">
            <div>
              <p class="eyebrow">GROUP</p>
              <h2>{{ activeGroup.name }} 群组权限</h2>
              <p>{{ activeGroup.fullPath }}</p>
              <p>{{ activeGroup.description || '暂无群组描述' }}</p>
              <p>创建时间: {{ activeGroup.creationDate ? dayjs(activeGroup.creationDate).format('YYYY-MM-DD HH:mm:ss') : '-' }}</p>
            </div>

            <Shield :size="22" color="#3569ed" />
          </div>

          <div class="stats-row">
            <div class="stat-item">
              <span>有效成员</span>
              <strong>{{ activeGroupMembers.length }}</strong>
            </div>

            <div class="stat-item">
              <span>直接成员</span>
              <strong>{{ directMembers(activeGroupMembers).length }}</strong>
            </div>

            <div class="stat-item">
              <span>继承成员</span>
              <strong>{{ inheritedMembers(activeGroupMembers).length }}</strong>
            </div>

            <div class="stat-item">
              <span>直接项目</span>
              <strong>{{ activeProjects.length }}</strong>
            </div>
          </div>
        </div>

        <!-- Group 成员权限 -->
        <div class="panel detail-panel">
          <div class="panel-head">
            <div>
              <h2>
                <Users :size="17" />
                群组有效成员
              </h2>
              <p>包含当前群组直接授权和从上级群组继承的成员</p>
            </div>
          </div>

          <table class="compact-table">
            <thead>
              <tr>
                <th>用户</th>
                <th>账号</th>
                <th>权限级别</th>
                <th>授权方式</th>
                <th>授权来源</th>
                <th>授权时间</th>
              </tr>
            </thead>

            <tbody>
              <tr v-for="m in activeGroupMembers" :key="m.id">
                <td>{{ m.userName }}</td>
                <td>@{{ m.username }}</td>
                <td>
                  <span :class="['tag', levelClass(m.accessLevel)]">
                    {{ levelName(m.accessLevel) }}
                  </span>
                </td>
                <td>
                  <span :class="m.inherited ? 'inherit-tag' : 'direct-tag'">
                    {{ m.inherited ? '继承' : '直接授权' }}
                  </span>
                </td>
                <td>
                  <span class="source-text" :title="sourceName(m)">
                    {{ sourceName(m) }}
                  </span>
                </td>
                <td>{{ m.grantedAt ? dayjs(m.grantedAt).format('YYYY-MM-DD HH:mm:ss') : '-' }}</td>
              </tr>

              <tr v-if="!activeGroupMembers.length">
                <td colspan="5" class="empty-cell">
                  该群组暂无有效成员
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Project 列表 -->
        <div class="project-section">
          <div class="section-title">
            <h2>群组下的项目</h2>
            <span>{{ activeProjects.length }} 个项目</span>
          </div>

          <div
            v-for="p in activeProjects"
            :key="p.id"
            class="panel project-panel"
          >
            <div class="panel-head">
                <div class="project-info">
                <div class="project-header">
                    <h2>
                    <FileCode2 :size="17" color="#178a70" />
                    <span>{{ p.name }}</span>
                    </h2>
                    <p class="project-path" :title="p.pathWithNS">
                    {{ p.pathWithNS }}
                    </p>
                </div>

                <p class="project-description" :title="p.description">
                    {{ p.description || '暂无项目描述' }}
                </p>

                <div class="project-meta">
                    <span>
                    <GitCommit :size="14" />
                    提交 {{ p.commitCount ?? 0 }} 次
                    </span>
                    <span>
                    <HardDrive :size="14" />
                    {{ (p.storageSize / 1024 / 1024).toFixed(2) }} MB
                    </span>
                    <span>
                    <CalendarDays :size="14" />
                    {{ p.creationDate ? dayjs(p.creationDate).format('YYYY-MM-DD') : '-' }}
                    </span>
                </div>
                </div>

              <a
                v-if="p.webUrl"
                :href="p.webUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="project-link"
              >
                GitLab
              </a>
            </div>

            <div class="project-stats">
              <span>
                有效成员
                <strong>{{ getProjectMembers(p.gitlabId).length }}</strong>
              </span>

              <span>
                直接成员
                <strong>
                  {{ directMembers(getProjectMembers(p.gitlabId)).length }}
                </strong>
              </span>

              <span>
                继承成员
                <strong>
                  {{ inheritedMembers(getProjectMembers(p.gitlabId)).length }}
                </strong>
              </span>
            </div>

            <table class="compact-table">
              <thead>
                <tr>
                  <th>用户</th>
                  <th>账号</th>
                  <th>权限级别</th>
                  <th>授权方式</th>
                  <th>授权来源</th>
                  <th>授权时间</th>
                </tr>
              </thead>

              <tbody>
                <tr
                  v-for="m in getProjectMembers(p.gitlabId)"
                  :key="m.id"
                >
                  <td>{{ m.userName }}</td>
                  <td>@{{ m.username }}</td>
                  <td>
                    <span :class="['tag', levelClass(m.accessLevel)]">
                      {{ levelName(m.accessLevel) }}
                    </span>
                  </td>
                  <td>
                    <span :class="m.inherited ? 'inherit-tag' : 'direct-tag'">
                      {{ m.inherited ? '继承' : '直接授权' }}
                    </span>
                  </td>
                  <td>
                    <span class="source-text" :title="sourceName(m)">
                      {{ sourceName(m) }}
                    </span>
                  </td>
                  <td>{{ m.grantedAt ? dayjs(m.grantedAt).format('YYYY-MM-DD HH:mm:ss') : '-' }}</td>
                </tr>

                <tr v-if="!getProjectMembers(p.gitlabId).length">
                  <td colspan="5" class="empty-cell">
                    暂无成员数据
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div
            v-if="activeProjects.length === 0"
            class="panel empty-state"
          >
            该群组下暂无直接归属的项目
          </div>
        </div>
      </main>

      <!-- 未选择群组 -->
      <main v-else class="panel empty-state main-panel">
        <Folder :size="36" />
        <p>请选择左侧群组查看权限</p>
      </main>
    </div>
  </div>
</template>


<style scoped>
/* ==============================
   页面整体布局
============================== */

.perm-layout {
  display: flex;
  gap: 17px;
  align-items: flex-start;
}

/* ==============================
   左侧群组面板
============================== */

.group-panel {
  width: 320px;
  flex-shrink: 0;
  position: sticky;
  top: 16px;
  overflow: hidden;
}

/* 群组标题栏 */

.group-panel-head {
  padding: 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border-bottom: 1px solid #edf0f5;
}

.group-title > div {
  display: flex;
  align-items: center;
  gap: 8px;
}

.group-title h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 650;
  color: #26344d;
  white-space: nowrap;
}

.group-count {
  padding: 3px 7px;
  border-radius: 10px;
  background: #f1f4f9;
  color: #8490a5;
  font-size: 11px;
  white-space: nowrap;
}

/* 展开、收起按钮 */

.tree-actions {
  display: flex;
  align-items: center;
  gap: 5px;
  flex-shrink: 0;
}

.tree-action-btn {
  width: 29px;
  height: 29px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 1px solid #e5eaf2;
  border-radius: 6px;
  background: #fff;
  color: #64748b;
  cursor: pointer;
  transition: all 0.15s ease;
}

.tree-action-btn:hover:not(:disabled) {
  color: #3569ed;
  background: #f2f6ff;
  border-color: #cbdafe;
}

.tree-action-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* 群组树区域 */

.group-tree {
  display: flex;
  flex-direction: column;
  padding: 10px 0 14px;
  max-height: calc(100vh - 240px);
  overflow-y: auto;
  overflow-x: hidden;
}

/* 群组树滚动条 */

.group-tree::-webkit-scrollbar {
  width: 5px;
}

.group-tree::-webkit-scrollbar-thumb {
  background: #dce3ed;
  border-radius: 5px;
}

.group-tree::-webkit-scrollbar-thumb:hover {
  background: #b9c6d9;
}

/* ==============================
   右侧权限详情
============================== */

.main-panel {
  flex: 1;
  min-width: 0;
}

.detail-panel,
.project-panel {
  margin-bottom: 17px;
}

/* 面板标题 */

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.panel-head h2 {
  display: flex;
  align-items: center;
  gap: 7px;
}

.panel-head p {
  margin-top: 5px;
}

/* ==============================
   统计信息
============================== */

.stats-row,
.project-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 30px;
  padding: 15px 18px;
  border-top: 1px solid #edf0f5;
}

.stat-item,
.project-stats span {
  display: flex;
  flex-direction: column;
  gap: 5px;
  color: #8490a5;
  font-size: 12px;
}

.stat-item strong,
.project-stats strong {
  color: #26344d;
  font-size: 19px;
}

/* ==============================
   成员权限表格
============================== */

.compact-table {
  width: 100%;
  border-collapse: collapse;
}

.compact-table th,
.compact-table td {
  padding: 10px 16px;
  text-align: left;
  border-bottom: 1px solid #edf0f5;
  font-size: 13px;
}

.compact-table th {
  color: #8490a5;
  font-weight: 500;
  background: #fafbfd;
}

.compact-table tbody tr {
  transition: background 0.15s ease;
}

.compact-table tbody tr:hover {
  background: #fafcff;
}

.compact-table tbody tr:last-child td {
  border-bottom: 0;
}

/* ==============================
   权限等级标签
============================== */

.tag,
.direct-tag,
.inherit-tag {
  display: inline-flex;
  align-items: center;
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 12px;
  white-space: nowrap;
}

.owner {
  background: #fce8e8;
  color: #c33c3c;
}

.maintainer {
  background: #fff0df;
  color: #b96a13;
}

.developer {
  background: #e6f6f3;
  color: #178a70;
}

.reporter {
  background: #e9efff;
  color: #3569ed;
}

.guest {
  background: #f0f1f4;
  color: #697386;
}

/* ==============================
   授权方式
============================== */

.direct-tag {
  background: #e6f6f3;
  color: #178a70;
}

.inherit-tag {
  background: #edf2ff;
  color: #3569ed;
}

/* 授权来源 */

.source-label {
  font-size: 13px;
}

.source-direct {
  color: #059669;
}

.source-inherited {
  color: #3568ed;
}

.source-unknown {
  color: #d97706;
}

/* 授权来源：超出宽度显示省略号 */
.source-text {
  display: inline-block;
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
  color: #64748b;
}

/* ==============================
   空数据状态
============================== */

.empty-cell {
  text-align: center !important;
  color: #999;
  padding: 25px !important;
}

.empty-state,
.empty-tree {
  text-align: center;
  padding: 35px;
  color: #8b95a7;
}

/* ==============================
   项目区域
============================== */

.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 22px 0 12px;
}

.section-title span {
  color: #8490a5;
  font-size: 13px;
}

.project-link {
  color: #3569ed;
  font-size: 13px;
  text-decoration: none;
}

.project-link:hover {
  text-decoration: underline;
}

.project-info {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.project-header h2 {
  display: flex;
  align-items: center;
  gap: 7px;
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
}

.project-path {
  margin: 5px 0 0;
  font-size: 12px;
  color: #94a3b8;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.project-description {
  margin: 0;
  font-size: 13px;
  color: #64748b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.project-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
  padding-top: 8px;
  border-top: 1px solid #f1f5f9;
  color: #64748b;
  font-size: 12px;
}

.project-meta span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

/* ==============================
   错误提示
============================== */

.error-panel {
  padding: 12px 16px;
  margin-bottom: 15px;
  color: #b42318;
  background: #fef3f2;
  border-radius: 7px;
}

/* ==============================
   加载动画
============================== */

.spinning {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

/* ==============================
   响应式布局
============================== */

@media (max-width: 1100px) {
  .group-panel {
    width: 290px;
  }

  .group-panel-head {
    padding: 13px;
  }

  .tree-action-btn {
    padding: 0 6px;
    font-size: 11px;
  }
}

@media (max-width: 900px) {
  .perm-layout {
    flex-direction: column;
  }

  .group-panel {
    width: 100%;
    position: static;
  }

  .group-tree {
    max-height: 350px;
  }

  .main-panel {
    width: 100%;
  }

  .compact-table {
    min-width: 650px;
  }

  .detail-panel,
  .project-panel {
    overflow-x: auto;
  }
}
</style>

