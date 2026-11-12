<script setup>
defineProps({
  label: String,
  value: [Number, String],
  suffix: String,
  tone: { type: String, default: 'blue' },
  note: String,
})
</script>

<template>
  <article class="stat-card">
    <div class="stat-label">{{ label }}</div>
    <div class="stat-value" :class="`tone-${tone}`">
      {{ Number(value || 0).toLocaleString() }}
      <small v-if="suffix">{{ suffix }}</small>
    </div>
    <div class="stat-note">{{ note }}</div>
  </article>
</template>