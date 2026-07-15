<script lang="ts">
	import { enhance } from '$app/forms';
	import { resolve } from '$app/paths';
	import { History, X } from '@lucide/svelte';
	import SearchResultRow from './SearchResultRow.svelte';
	import type { RecentSearchItem } from '$lib/server/api/search';

	let { recent }: { recent: RecentSearchItem[] } = $props();

	// Writable derived: a remove/clear overrides this locally for an
	// optimistic update, discarded in favor of the new `recent` prop once
	// navigation reloads it (same pattern as +page.svelte's `inputValue`).
	let items = $derived(recent);

	// Returns a restore callback that puts the item back at its original
	// index, for reverting on a failed submission — mirrors
	// SearchDiscovery.svelte's follow/unfollow revert-on-failure pattern.
	function removeLocally(id: string): (() => void) | null {
		const index = items.findIndex((item) => item.id === id);
		const removed = items[index];
		if (index === -1 || !removed) return null;
		items = items.filter((item) => item.id !== id);
		return () => {
			items = [...items.slice(0, index), removed, ...items.slice(index)];
		};
	}

	function clearLocally(): () => void {
		const previous = items;
		items = [];
		return () => {
			items = previous;
		};
	}
</script>

{#if items.length > 0}
	<div class="flex flex-col gap-3">
		<div class="flex items-center justify-between">
			<h2 class="text-sm font-bold text-base-content/60 uppercase tracking-wide">Recent</h2>
			<form
				method="POST"
				action="?/clearRecent"
				use:enhance={() => {
					const restore = clearLocally();
					return async ({ result }) => {
						if (result.type === 'error' || result.type === 'failure') restore();
					};
				}}
			>
				<button
					type="submit"
					class="text-xs font-bold text-base-content/60 transition-colors hover:text-base-content"
				>
					Clear all
				</button>
			</form>
		</div>
		<ul class="flex flex-col gap-1">
			{#each items as row (row.id)}
				<li class="flex items-center gap-1">
					<div class="min-w-0 flex-1">
						{#if row.type === 'users'}
							<SearchResultRow row={{ type: 'users', item: row.item }} />
						{:else if row.type === 'hashtags'}
							<SearchResultRow row={{ type: 'hashtags', item: row.item }} />
						{:else}
							<a
								href={resolve(`/search?q=${encodeURIComponent(row.item)}`)}
								class="flex items-center gap-3 rounded-2xl border border-base-300 bg-base-100 p-3 transition-colors hover:bg-base-200"
							>
								<span
									class="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-base-300 text-base-content/60"
								>
									<History class="h-5 w-5" />
								</span>
								<span class="min-w-0 flex-1 truncate font-bold">{row.item}</span>
							</a>
						{/if}
					</div>
					<form
						method="POST"
						action="?/removeRecent"
						use:enhance={() => {
							const restore = removeLocally(row.id);
							return async ({ result }) => {
								if ((result.type === 'error' || result.type === 'failure') && restore) restore();
							};
						}}
					>
						<input type="hidden" name="id" value={row.id} />
						<button
							type="submit"
							aria-label="Remove from recent searches"
							class="grid h-8 w-8 shrink-0 place-items-center rounded-full text-base-content/40 transition-colors hover:bg-base-300 hover:text-base-content"
						>
							<X class="h-4 w-4" />
						</button>
					</form>
				</li>
			{/each}
		</ul>
	</div>
{/if}
