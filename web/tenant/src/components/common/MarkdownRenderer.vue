<script setup lang="ts">
import { computed } from 'vue'
import { marked } from 'marked'

marked.setOptions({
	breaks: true,
	gfm: true,
})

const props = defineProps<{
	content: string
}>()

const renderedHtml = computed(() => {
	if (!props.content) return ''
	return marked.parse(props.content) as string
})
</script>

<template>
	<div class="markdown-content" v-html="renderedHtml" />
</template>
