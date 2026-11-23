<script setup lang="ts">
// <Method m="GET" /> — an HTTP method in the TUI's colors.
defineProps<{ m: string }>()
</script>

<template>
  <span class="t-method" :class="m.toLowerCase()">{{ m.toUpperCase() }}</span>
</template>
