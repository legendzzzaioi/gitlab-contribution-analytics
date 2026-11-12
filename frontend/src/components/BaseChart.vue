<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { init, use } from 'echarts/core'
import { LineChart, BarChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

use([LineChart, BarChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

const props = defineProps({
  option: { type: Object, required: true },
})

const element = ref(null)
let chart

// 渲染图表
function render() {
  if (chart && props.option) {
    chart.setOption(props.option, { notMerge: true, lazyUpdate: true })
  }
}

// 调整图表大小
function resize() {
  chart?.resize()
}

// 组件挂载时初始化图表
onMounted(async () => {
  await nextTick()
  chart = init(element.value)
  render()
  window.addEventListener('resize', resize)
})

// 监听选项变化并重新渲染
watch(() => props.option, render, { deep: true })

// 组件卸载时清理资源
onBeforeUnmount(() => {
  window.removeEventListener('resize', resize)
  chart?.dispose()
})
</script>

<template>
  <div ref="element" class="chart-canvas" role="img" aria-label="数据图表"></div>
</template>