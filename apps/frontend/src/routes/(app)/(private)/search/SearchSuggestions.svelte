<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { typeaheadNav } from '$lib/utils/typeaheadNav';
	import { interleaveSuggestions, type SuggestionItem } from '$lib/utils/interleaveSuggestions';
	import { recordRecentSearch } from '$lib/utils/recentSearch';
	import SearchDropdownPanel from './SearchDropdownPanel.svelte';
	import SearchResultRow from './SearchResultRow.svelte';
	import type { UserSuggestion, HashtagSuggestion } from '$lib/server/api/search';

	let {
		users,
		hashtags,
		onclose
	}: {
		users: UserSuggestion[];
		hashtags: HashtagSuggestion[];
		onclose: () => void;
	} = $props();

	let flat = $derived(interleaveSuggestions(users, hashtags));

	function rowKey(row: SuggestionItem): string {
		if (row.type === 'users') return `users-${row.item.username}`;
		return `hashtags-${row.item.name}`;
	}

	let activeIndex = $state(0);
	// Only true once the user has explicitly moved the highlight with arrow
	// keys; gates whether Enter selects a suggestion or reaches the form's
	// native submit for a plain free-text search.
	let hasHighlighted = $state(false);

	// Reset navigation state whenever the suggestion set changes.
	$effect(() => {
		if (flat) {
			activeIndex = 0;
			hasHighlighted = false;
		}
	});

	function recordSuggestion(row: SuggestionItem) {
		if (row.type === 'users') recordRecentSearch('users', row.item.username);
		else recordRecentSearch('hashtags', row.item.name);
	}

	function activate(row: SuggestionItem | undefined) {
		if (!row) return;
		onclose();
		recordSuggestion(row);
		if (row.type === 'users') goto(resolve(`/@${row.item.username}`));
		else goto(resolve(`/search?q=%23${encodeURIComponent(row.item.name)}`));
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !hasHighlighted) return;
		if (e.key === 'ArrowDown' || e.key === 'ArrowUp') hasHighlighted = true;
		const { index, action } = typeaheadNav(e.key, activeIndex, flat.length);
		if (action === 'none' && index === activeIndex) return;
		e.preventDefault();
		activeIndex = index;
		if (action === 'select') activate(flat[index]);
		else if (action === 'clear') onclose();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if flat.length > 0}
	<SearchDropdownPanel>
		<ul class="menu menu-sm w-full gap-1 p-0">
			{#each flat as row, i (rowKey(row))}
				<li>
					<SearchResultRow
						{row}
						active={i === activeIndex}
						onmouseenter={() => (activeIndex = i)}
						onclick={() => {
							recordSuggestion(row);
							onclose();
						}}
					/>
				</li>
			{/each}
		</ul>
	</SearchDropdownPanel>
{/if}
