<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { Search, X } from '@lucide/svelte';
	import { createPagination } from '$lib/createPagination.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import LoadMoreButton from '$lib/components/LoadMoreButton.svelte';
	import { fetchCursorPage } from '$lib/utils/clientFetch';
	import { recordRecentSearch } from '$lib/utils/recentSearch';
	import RecentSearches from './RecentSearches.svelte';
	import SearchDiscovery from './SearchDiscovery.svelte';
	import SearchPostThumbnail from './SearchPostThumbnail.svelte';
	import SearchSuggestions from './SearchSuggestions.svelte';
	import type { PageData } from './$types';
	import type {
		SearchAllItem,
		SearchPostItem,
		UserSuggestion,
		HashtagSuggestion
	} from '$lib/server/api/search';
	import { searchQueryPrefix, stripSearchQueryPrefix } from '$lib/utils/searchQuery';

	type InternalPath = `/${string}`;

	const SUGGEST_DEBOUNCE_MS = 150;

	let { data }: { data: PageData } = $props();

	let inputEl = $state<HTMLInputElement | null>(null);
	// Writable derived: typing overrides this value locally, and the override
	// is discarded in favor of the new data.q once navigation changes it.
	let inputValue = $derived(data.q);
	let suggestUsers = $state<UserSuggestion[]>([]);
	let suggestHashtags = $state<HashtagSuggestion[]>([]);
	let debounceTimer: ReturnType<typeof setTimeout> | undefined;
	let requestToken = 0;

	// True only once the input has been focused; gates the recent-searches
	// dropdown so it appears on focus rather than as static page content.
	let inputFocused = $state(false);

	// Same writable-derived pattern as inputValue above; lifted here (rather than owned
	// by RecentSearches) so showRecentDropdown below reacts to a remove/clear immediately.
	let recentItems = $derived(data.recent);
	let showRecentDropdown = $derived(inputFocused && !inputValue && recentItems.length > 0);
	let anyDropdownOpen = $derived(
		showRecentDropdown || suggestUsers.length > 0 || suggestHashtags.length > 0
	);

	function closeSuggestions() {
		// Bump the token so any fetch already in flight is discarded on arrival
		// instead of repopulating the dropdown after it was dismissed.
		requestToken++;
		suggestUsers = [];
		suggestHashtags = [];
	}

	// Closes both dropdowns on focus leaving the whole widget, not on input blur alone,
	// since clicking into either dropdown blurs the input first.
	function handleWidgetFocusOut(e: FocusEvent) {
		const next = e.relatedTarget as Node | null;
		if (!next || !(e.currentTarget as HTMLElement).contains(next)) {
			inputFocused = false;
			closeSuggestions();
		}
	}

	// Returns a restore callback for reverting on a failed submission. Refocuses the input
	// first: the remove button holds focus and is removed from the DOM once `recentItems`
	// updates, and focus falling to <body> would trip handleWidgetFocusOut and close the
	// whole dropdown instead of just this row.
	function removeRecentLocally(id: string): (() => void) | null {
		const index = recentItems.findIndex((item) => item.id === id);
		const removed = recentItems[index];
		if (index === -1 || !removed) return null;
		recentItems = recentItems.filter((item) => item.id !== id);
		inputEl?.focus();
		return () => {
			recentItems = [...recentItems.slice(0, index), removed, ...recentItems.slice(index)];
		};
	}

	// Same focus caveat as removeRecentLocally above: the Clear all button
	// holds focus when clicked and is removed once the list empties.
	function clearRecentLocally(): () => void {
		const previous = recentItems;
		recentItems = [];
		inputEl?.focus();
		return () => {
			recentItems = previous;
		};
	}

	function buildSearchUrl(q: string): InternalPath {
		return `/search?${new URLSearchParams({ q }).toString()}`;
	}

	async function fetchSuggestions(rawQuery: string) {
		const token = ++requestToken;
		const prefix = searchQueryPrefix(rawQuery);
		const query = stripSearchQueryPrefix(rawQuery, prefix);
		if (!query) {
			closeSuggestions();
			return;
		}

		const wantUsers = prefix !== '#';
		const wantHashtags = prefix !== '@';

		try {
			const [usersRes, hashtagsRes] = await Promise.all([
				wantUsers ? fetch(`/suggest?type=users&q=${encodeURIComponent(query)}`) : null,
				wantHashtags ? fetch(`/suggest?type=hashtags&q=${encodeURIComponent(query)}`) : null
			]);

			const users = usersRes?.ok ? ((await usersRes.json()) as UserSuggestion[]) : [];
			const hashtags = hashtagsRes?.ok ? ((await hashtagsRes.json()) as HashtagSuggestion[]) : [];

			// Only stale-check once, after every await point, so an out-of-order
			// response can never partially overwrite fresher suggestions.
			if (token !== requestToken) return;
			suggestUsers = users;
			suggestHashtags = hashtags;
		} catch {
			if (token === requestToken) closeSuggestions();
		}
	}

	// Blur clears suggestions, so refocusing must re-fetch, not wait for a keystroke.
	function onFocus() {
		inputFocused = true;
		if (inputValue) fetchSuggestions(inputValue);
	}

	function onInput(e: Event) {
		const value = (e.currentTarget as HTMLInputElement).value;
		clearTimeout(debounceTimer);
		if (!value) {
			closeSuggestions();
			// Clearing the field returns to discovery.
			if (data.q) goto(resolve('/search'));
			return;
		}
		debounceTimer = setTimeout(() => fetchSuggestions(value), SUGGEST_DEBOUNCE_MS);
	}

	function clearInput() {
		clearTimeout(debounceTimer);
		closeSuggestions();
		inputValue = '';
		inputEl?.focus();
		if (data.q) goto(resolve('/search'));
	}

	function onSubmit(e: SubmitEvent) {
		e.preventDefault();
		clearTimeout(debounceTimer);
		closeSuggestions();
		// Trimmed once, up front: a whitespace-only submission behaves like an empty one,
		// and the trimmed value is what gets recorded, searched, and shown in the URL/title.
		const value = (inputEl?.value ?? '').trim();
		if (!value) return;
		// Recorded with prefix included so it replays to the same /search?q=<value> page.
		recordRecentSearch('posts', value);
		goto(resolve(buildSearchUrl(value)));
	}

	// Results arrive in the shared blended shape; this view renders posts only.
	function toPostsPage(page: { items: SearchAllItem[]; nextCursor: string | null }): {
		items: SearchPostItem[];
		nextCursor: string | null;
	} {
		return {
			items: page.items
				.filter((row): row is Extract<SearchAllItem, { type: 'posts' }> => row.type === 'posts')
				.map((row) => row.item),
			nextCursor: page.nextCursor
		};
	}

	const resultsPagination = createPagination(
		() => toPostsPage(data.results),
		(cursor) =>
			fetchCursorPage<SearchAllItem>(
				fetch,
				`/search?q=${encodeURIComponent(data.resultsQuery)}&type=posts`,
				cursor
			).then(toPostsPage)
	);
</script>

<svelte:head>
	<title>{data.q ? `"${data.q}" — Search` : 'Search'} — Phasma</title>
</svelte:head>

<div class="mx-auto flex max-w-xl flex-col gap-6">
	<div class="relative z-10 w-full" onfocusout={handleWidgetFocusOut}>
		<form class="relative w-full" onsubmit={onSubmit}>
			<Search
				class="pointer-events-none absolute top-1/2 left-4 z-10 h-4 w-4 -translate-y-1/2 text-base-content"
			/>
			<input
				bind:this={inputEl}
				type="search"
				class="input w-full border-base-300 pr-14 pl-11 [&::-webkit-search-cancel-button]:hidden {anyDropdownOpen
					? 'rounded-t-2xl rounded-b-none'
					: 'rounded-full'}"
				placeholder="Search users, posts, hashtags…"
				bind:value={inputValue}
				oninput={onInput}
				onfocus={onFocus}
				aria-label="Search"
			/>
			{#if inputValue}
				<button
					type="button"
					onclick={clearInput}
					class="absolute top-1/2 right-4 inline-flex h-8 w-8 -translate-y-1/2 items-center justify-center rounded-full text-base-content transition-colors hover:bg-base-300"
					aria-label="Clear search"
				>
					<X class="h-4 w-4" />
				</button>
			{/if}
		</form>
		{#if anyDropdownOpen}
			<!-- border-t-0 (vs. border-t-transparent) would mismatch border width there,
				rendering a visible seam at the rounded bottom corners in Chromium. -->
			<div
				class="absolute top-full w-full rounded-b-2xl border border-t-transparent border-base-300 bg-base-100 shadow-lg shadow-slate-900/10"
			>
				<SearchSuggestions
					users={suggestUsers}
					hashtags={suggestHashtags}
					onclose={closeSuggestions}
				/>
				{#if showRecentDropdown}
					<RecentSearches
						items={recentItems}
						onRemove={removeRecentLocally}
						onClear={clearRecentLocally}
					/>
				{/if}
			</div>
		{/if}
	</div>

	<div>
		{#if !data.q}
			<SearchDiscovery
				hasRecent={recentItems.length > 0}
				suggested={data.suggested}
				popular={data.popular}
			/>
		{:else if resultsPagination.items.length === 0}
			<EmptyState
				icon="triangle-alert"
				title="No results"
				description="Nothing matched &ldquo;{data.q}&rdquo;. Try a different query."
			/>
		{:else}
			<section class="flex flex-col gap-3">
				<div class="grid grid-cols-3 gap-2">
					{#each resultsPagination.items as post (post.id)}
						<SearchPostThumbnail {post} />
					{/each}
				</div>
				{#if !resultsPagination.done}
					<div class="flex flex-col items-center gap-2">
						{#if resultsPagination.error}
							<p class="text-sm text-error">{resultsPagination.error}</p>
						{/if}
						<LoadMoreButton loading={resultsPagination.loading} onclick={resultsPagination.more} />
					</div>
				{/if}
			</section>
		{/if}
	</div>
</div>
