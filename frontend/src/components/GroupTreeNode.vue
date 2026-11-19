
<script setup>
import { computed } from 'vue'
import {
  ChevronRight,
  ChevronDown,
  Folder,
  FolderOpen
} from 'lucide-vue-next'

defineOptions({
  name: 'GroupTreeNode'
})

const props = defineProps({
  node: {
    type: Object,
    required: true
  },
  activeGroupId: {
    type: [Number, String],
    default: null
  },
  expandedKeys: {
    type: Object,
    required: true
  }
})

const emit = defineEmits(['select', 'toggle'])

// 获取组ID
const groupId = computed(() => Number(props.node.gitlabId))

// 检查是否有子节点
const hasChildren = computed(() => {
  return props.node.children?.length > 0
})

// 检查是否展开
const expanded = computed(() => {
  return props.expandedKeys.has(groupId.value)
})

// 检查是否激活
const active = computed(() => {
  return Number(props.activeGroupId) === groupId.value
})

// 选择节点
function selectNode() {
  emit('select', props.node)
}

// 切换节点展开/收缩
function toggleNode(event) {
  event.stopPropagation()
  emit('toggle', groupId.value)
}
</script>

<template>
  <div class="tree-node">
    <div
      class="tree-node-row"
      :class="{ active: active }"
      @click="selectNode"
    >
      <button
        v-if="hasChildren"
        class="tree-toggle"
        @click="toggleNode"
        :title="expanded ? '收起子群组' : '展开子群组'"
      >
        <ChevronDown v-if="expanded" :size="15" />
        <ChevronRight v-else :size="15" />
      </button>

      <span v-else class="tree-toggle-placeholder"></span>

      <component
        :is="expanded && hasChildren ? FolderOpen : Folder"
        class="tree-folder-icon"
        :size="17"
      />

      <span class="tree-node-name" :title="node.fullPath">
        {{ node.name }}
      </span>

      <span class="tree-node-id">
        {{ node.description }}
      </span>
    </div>

    <div
      v-if="hasChildren && expanded"
      class="tree-children"
    >
      <GroupTreeNode
        v-for="child in node.children"
        :key="child.gitlabId"
        :node="child"
        :active-group-id="activeGroupId"
        :expanded-keys="expandedKeys"
        @select="emit('select', $event)"
        @toggle="emit('toggle', $event)"
      />
    </div>
  </div>
</template>

<style scoped>
.tree-node {
  position: relative;
}

.tree-node-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  height: 37px;
  margin: 2px 9px;
  padding: 0 9px;
  border-radius: 7px;
  cursor: pointer;
  color: #34415b;
  transition: background 0.15s ease, color 0.15s ease;
}

.tree-node-row:hover {
  background: #f2f5fb;
}

.tree-node-row.active {
  color: #2864dc;
  background: #eaf1ff;
  font-weight: 600;
}

.tree-node-row.active::before {
  content: '';
  position: absolute;
  left: -9px;
  top: 6px;
  bottom: 6px;
  width: 3px;
  border-radius: 0 3px 3px 0;
  background: #3975ed;
}

.tree-toggle {
  display: grid;
  place-items: center;
  width: 17px;
  height: 20px;
  padding: 0;
  border: 0;
  background: transparent;
  color: #8b98ad;
  cursor: pointer;
  flex-shrink: 0;
}

.tree-toggle:hover {
  color: #3975ed;
}

.tree-toggle-placeholder {
  width: 17px;
  flex-shrink: 0;
}

.tree-folder-icon {
  flex-shrink: 0;
  color: #4384ee;
  fill: #4384ee;
  fill-opacity: 0.12;
}

.tree-node-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}

.tree-node-id {
  flex-shrink: 0;
  padding: 2px 6px;
  border-radius: 10px;
  background: #f0f4fa;
  color: #8996aa;
  font-size: 10px;
  font-weight: 400;
}

.tree-node-row.active .tree-node-id {
  background: #dce8ff;
  color: #3975ed;
}

.tree-children {
  position: relative;
  margin-left: 19px;
  padding-left: 10px;
}

.tree-children::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 15px;
  border-left: 1px solid #e4eaf3;
}
</style>
