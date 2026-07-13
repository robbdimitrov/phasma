<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { navigating } from '$app/state';
	import type { LayoutData } from './$types';
	import { setThemeContext } from '$lib/theme.svelte';

	let { data, children }: { data: LayoutData; children: import('svelte').Snippet } = $props();

	const themeController = setThemeContext(() => data.theme);

	onMount(() => themeController.start());
</script>

{#if navigating.to}
	<div class="fixed inset-x-0 top-0 z-50 h-0.5 overflow-hidden bg-primary/20">
		<div class="h-full w-1/3 origin-left rounded-full bg-primary animate-nav-progress"></div>
	</div>
{/if}

{@render children()}
