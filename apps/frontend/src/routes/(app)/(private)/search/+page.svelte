<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { createPagination } from '$lib/createPagination.svelte';
	import LoadMoreButton from '$lib/components/LoadMoreButton.svelte';
	import { fetchCursorPage } from '$lib/utils/clientFetch';
	import SearchDiscovery from './SearchDiscovery.svelte';
	import SearchResultList from './SearchResultList.svelte';
	import type { PageData } from './$types';
	import type { SearchItem, SearchType } from '$lib/server/api/search';

	type InternalPath = `/${string}`;

	let { data }: { data: PageData } = $props();

	const TABS: { label: string; value: SearchType }[] = [
		{ label: 'Posts', value: 'posts' },
		{ label: 'Users', value: 'users' },
		{ label: 'Hashtags', value: 'hashtags' }
	];

	function buildUrl(q: string, type: SearchType): InternalPath {
		const params = new URLSearchParams({ q, type });
		return `/search?${params.toString()}`;
	}

	function navigate(q: string, type: SearchType) {
		goto(resolve(buildUrl(q, type)), { replaceState: true });
	}

	let debounceTimer: ReturnType<typeof setTimeout> | undefined;

	function onInput(e: Event) {
		const value = (e.currentTarget as HTMLInputElement).value;
		clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			const resolvedType = value.startsWith('@')
				? 'users'
				: value.startsWith('#')
					? 'hashtags'
					: data.type;
			navigate(value, resolvedType);
		}, 300);
	}

	function onTabClick(type: SearchType) {
		navigate(data.q, type);
	}

	const pagination = createPagination(
		() => ({ items: data.items, nextCursor: data.nextCursor }),
		async (cursor) => {
			const params = new URLSearchParams({ q: data.q, type: data.type });
			return fetchCursorPage<SearchItem>(fetch, `/search?${params.toString()}`, cursor);
		}
	);
</script>

<svelte:head>
	<title>{data.q ? `"${data.q}" — Search` : 'Search'} — Phasma</title>
</svelte:head>

<div class="mx-auto flex max-w-xl flex-col gap-6">
	<div class="relative w-full">
		<input
			type="search"
			class="input input-bordered w-full rounded-full pl-4 pr-10"
			placeholder="Search users, posts, hashtags…"
			value={data.q}
			oninput={onInput}
			aria-label="Search"
		/>
	</div>

	{#if !data.q}
		<SearchDiscovery suggested={data.suggested} popular={data.popular} />
	{:else}
		<div role="tablist" class="tabs tabs-bordered justify-center font-bold">
			{#each TABS as tab (tab.value)}
				<button
					role="tab"
					class="tab"
					class:tab-active={data.type === tab.value}
					onclick={() => onTabClick(tab.value)}
					aria-selected={data.type === tab.value}
				>
					{tab.label}
				</button>
			{/each}
		</div>

		<SearchResultList items={pagination.items} q={data.q} type={data.type} />

		{#if !pagination.done && pagination.items.length > 0}
			<div class="flex flex-col items-center gap-2">
				{#if pagination.error}
					<p class="text-sm text-error">{pagination.error}</p>
				{/if}
				<LoadMoreButton loading={pagination.loading} onclick={pagination.more} />
			</div>
		{/if}
	{/if}
</div>
