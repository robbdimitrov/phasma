<script lang="ts">
	// Sheet chrome lives in +page.svelte, spanning this and the input above.
	let { children }: { children: import('svelte').Snippet } = $props();
</script>

<div class="p-2">
	{@render children()}
</div>
