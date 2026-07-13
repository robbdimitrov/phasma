<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { typeaheadNav } from '$lib/utils/typeaheadNav';
	import { interleaveSuggestions } from '$lib/utils/interleaveSuggestions';
	import SearchResultRow from './SearchResultRow.svelte';
	import type { UserSuggestion, HashtagSuggestion, SearchPostItem, SearchAllItem } from '$lib/server/api/search';

	let {
		users,
		posts,
		hashtags,
		onclose
	}: {
		users: UserSuggestion[];
		posts: SearchPostItem[];
		hashtags: HashtagSuggestion[];
		onclose: () => void;
	} = $props();

	let flat = $derived(interleaveSuggestions(users, posts, hashtags));

	function rowKey(row: SearchAllItem): string {
		if (row.type === 'users') return `users-${row.item.username}`;
		if (row.type === 'posts') return `posts-${row.item.id}`;
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

	function activate(row: SearchAllItem | undefined) {
		if (!row) return;
		onclose();
		if (row.type === 'users') {
			goto(resolve(`/@${row.item.username}`));
		} else if (row.type === 'posts') {
			goto(resolve(`/posts/${row.item.id}`));
		} else {
			goto(resolve(`/search?q=%23${encodeURIComponent(row.item.name)}`));
		}
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

	function handleFocusOut(e: FocusEvent) {
		const next = e.relatedTarget as Node | null;
		if (!next || !(e.currentTarget as HTMLElement).contains(next)) onclose();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if flat.length > 0}
	<div
		class="absolute top-full z-10 mt-1 w-full rounded-box border border-base-300 bg-base-100 p-2 shadow-lg shadow-slate-900/10"
		onfocusout={handleFocusOut}
	>
		<ul class="menu menu-sm w-full gap-1 p-0">
			{#each flat as row, i (rowKey(row))}
				<li>
					<SearchResultRow
						{row}
						active={i === activeIndex}
						onmouseenter={() => (activeIndex = i)}
						onclick={onclose}
					/>
				</li>
			{/each}
		</ul>
	</div>
{/if}
