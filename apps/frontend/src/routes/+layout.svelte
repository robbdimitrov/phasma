<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import type { LayoutData } from './$types';
	import { setThemeContext } from '$lib/theme.svelte';

	let { data, children }: { data: LayoutData; children: import('svelte').Snippet } = $props();

	const themeController = setThemeContext(() => data.theme);

	onMount(() => themeController.start());
</script>

{@render children()}
