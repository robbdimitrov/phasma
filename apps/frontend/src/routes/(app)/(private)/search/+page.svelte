<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { X } from '@lucide/svelte';
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

	// Writable derived: a remove/clear overrides this locally for an
	// optimistic update, discarded in favor of the new data.recent once
	// navigation reloads it (same pattern as `inputValue` above). Lifted here
	// rather than owned by RecentSearches so showRecentDropdown below reacts
	// to it immediately.
	let recentItems = $derived(data.recent);
	let showRecentDropdown = $derived(inputFocused && !inputValue && recentItems.length > 0);
	// Gates the scrim below: true whenever either dropdown (recents or live
	// suggestions) is actually showing content, so the page behind reads as
	// dimmed/inactive rather than the dropdown just looking like it's
	// clipping into unrelated content.
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

	// Closes both dropdowns once focus leaves the whole search widget (input,
	// live suggestions, or the recent-searches panel) rather than on input
	// blur alone, since clicking into either dropdown blurs the input first.
	function handleWidgetFocusOut(e: FocusEvent) {
		const next = e.relatedTarget as Node | null;
		if (!next || !(e.currentTarget as HTMLElement).contains(next)) {
			inputFocused = false;
			closeSuggestions();
		}
	}

	// Returns a restore callback that puts the item back at its original
	// index, for reverting on a failed submission — mirrors
	// SearchDiscovery.svelte's follow/unfollow revert-on-failure pattern.
	//
	// Refocusing the input here matters: the remove button itself holds focus
	// when clicked, and Svelte's reactive DOM update removes that button (its
	// row is gone from `recentItems`) shortly after this returns. Without
	// explicitly moving focus back to the input first, focus would fall back
	// to <body> when the button is detached, which handleWidgetFocusOut reads
	// as "focus left the widget" and closes the whole dropdown — including
	// any remaining items — instead of just the one row.
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
		// Trimmed once, up front: a whitespace-only submission should behave
		// like an empty one, and the trimmed value is what gets recorded,
		// searched, and shown in the URL/title (matching the backend, which
		// trims before validating).
		const value = (inputEl?.value ?? '').trim();
		if (!value) return;
		// Recorded as-is (prefix included): this always lands on
		// /search?q=<value>, never a profile/hashtag page directly, so it must
		// replay to the same posts-results page it originally produced.
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
			<input
				bind:this={inputEl}
				type="search"
				class="input w-full rounded-full pr-14 [&::-webkit-search-cancel-button]:hidden"
				placeholder="Search users, posts, hashtags…"
				bind:value={inputValue}
				oninput={onInput}
				onfocus={() => (inputFocused = true)}
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
		<SearchSuggestions users={suggestUsers} hashtags={suggestHashtags} onclose={closeSuggestions} />
		{#if showRecentDropdown}
			<RecentSearches
				items={recentItems}
				onRemove={removeRecentLocally}
				onClear={clearRecentLocally}
			/>
		{/if}
	</div>

	<div class="relative">
		{#if anyDropdownOpen}
			<!-- Dims the discovery/results content so an open dropdown reads as a
			     focused overlay rather than clipping over unrelated content.
			     Sized via inset-0 to this wrapper's own content height (not a
			     fixed guess) so it never over- or under-covers, and never
			     inflates the page's scrollable area the way an oversized
			     position: absolute height would. -->
			<div class="absolute inset-0 z-0 bg-base-100/70 backdrop-blur-sm" aria-hidden="true"></div>
		{/if}
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
