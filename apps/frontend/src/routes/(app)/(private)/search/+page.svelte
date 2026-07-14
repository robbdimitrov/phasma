<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { createPagination } from '$lib/createPagination.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import LoadMoreButton from '$lib/components/LoadMoreButton.svelte';
	import { fetchCursorPage } from '$lib/utils/clientFetch';
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
	let suggestUsers = $state<UserSuggestion[]>([]);
	let suggestHashtags = $state<HashtagSuggestion[]>([]);
	let debounceTimer: ReturnType<typeof setTimeout> | undefined;
	let requestToken = 0;

	// Hide discovery/results content while the dropdown is open — it's
	// absolutely positioned and would otherwise overlap the content below it.
	let suggestionsOpen = $derived(suggestUsers.length > 0 || suggestHashtags.length > 0);

	function closeSuggestions() {
		// Bump the token so any fetch already in flight is discarded on arrival
		// instead of repopulating the dropdown after it was dismissed.
		requestToken++;
		suggestUsers = [];
		suggestHashtags = [];
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
			// Clearing the field (backspace or the native search-input "x")
			// reverts to the discovery view, matching Instagram/Twitter/YouTube.
			if (data.q) goto(resolve('/search'));
			return;
		}
		debounceTimer = setTimeout(() => fetchSuggestions(value), SUGGEST_DEBOUNCE_MS);
	}

	function onSubmit(e: SubmitEvent) {
		e.preventDefault();
		clearTimeout(debounceTimer);
		closeSuggestions();
		const value = inputEl?.value ?? '';
		if (!value) return;
		goto(resolve(buildSearchUrl(value)));
	}

	// Results are always posts (see +page.server.ts); items still arrive
	// wrapped in the shared blended-item shape, so unwrap to plain posts here.
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
	<form class="relative w-full" onsubmit={onSubmit}>
		<input
			bind:this={inputEl}
			type="search"
			class="input input-bordered w-full rounded-full"
			placeholder="Search users, posts, hashtags…"
			value={data.q}
			oninput={onInput}
			aria-label="Search"
		/>
		<SearchSuggestions users={suggestUsers} hashtags={suggestHashtags} onclose={closeSuggestions} />
	</form>

	{#if !suggestionsOpen}
		{#if !data.q}
			<SearchDiscovery suggested={data.suggested} popular={data.popular} />
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
	{/if}
</div>
