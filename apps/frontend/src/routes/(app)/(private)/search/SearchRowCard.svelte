<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		tag,
		href,
		active = false,
		onclick,
		onmouseenter,
		children
	}: {
		tag: 'li' | 'a';
		href?: string;
		active?: boolean;
		onclick?: () => void;
		onmouseenter?: () => void;
		children: Snippet;
	} = $props();
</script>

<!-- onclick/onmouseenter only ever accompany tag="a"; svelte-check can't verify that statically. -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<svelte:element
	this={tag}
	{href}
	{onclick}
	{onmouseenter}
	class="flex items-center gap-3 rounded-2xl border border-base-300 bg-base-100 p-3 transition-colors hover:bg-base-200 {active
		? 'bg-base-200'
		: ''}"
>
	{@render children()}
</svelte:element>
