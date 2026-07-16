<script lang="ts">
	// Shared shell for the input's dropdown: both the live-suggestions
	// typeahead (SearchSuggestions.svelte) and the recent-searches panel
	// (RecentSearches.svelte) render into this so the two mutually-exclusive
	// modes read as one consistent panel rather than two hand-matched copies.
	let { children }: { children: import('svelte').Snippet } = $props();
</script>

<div
	class="absolute top-full z-10 mt-1 w-full rounded-box border border-base-300 bg-base-100 p-2 shadow-lg shadow-slate-900/10"
>
	{@render children()}
</div>
