<script setup lang="ts">
// <Pane title="…"> — a rounded pane with its title in the border, like the TUI.
defineProps<{ title?: string }>()
</script>

<template>
  <div class="t-pane">
    <span v-if="title" class="t-pane-title">{{ title }}</span>
    <slot />
  </div>
</template>
