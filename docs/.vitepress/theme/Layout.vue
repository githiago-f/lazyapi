<script setup lang="ts">
import DefaultTheme from "vitepress/theme"
import HelpBar from "./components/HelpBar.vue"
import Hero from "./components/Hero.vue"
import NotFound from "./components/NotFound.vue"

const { Layout } = DefaultTheme
</script>

<template>
  <Layout>
    <template #nav-bar-title-before>
      <span class="t-prompt">&gt;_</span>
    </template>
    <template #nav-bar-title-after>
      <span class="t-sep">·</span>
      <span class="t-sub">docs</span>
    </template>
    <template #home-hero-before>
      <Hero />
    </template>
    <template #not-found>
      <NotFound />
    </template>
    <template #layout-bottom>
      <HelpBar />
    </template>
  </Layout>
</template>
