<script lang="ts">
	import { enhance } from '$app/forms';
	import { resolve } from '$app/paths';
	import { History, X } from '@lucide/svelte';
	import SearchResultRow from './SearchResultRow.svelte';
	import type { RecentSearchItem } from '$lib/server/api/search';

	// `items` and its mutators live in the parent (+page.svelte): it also
	// gates whether this dropdown is shown at all, so it needs to observe an
	// optimistic remove/clear immediately rather than through a prop that only
	// updates on the next navigation.
	let {
		items,
		onRemove,
		onClear
	}: {
		items: RecentSearchItem[];
		onRemove: (id: string) => (() => void) | null;
		onClear: () => () => void;
	} = $props();
</script>

{#if items.length > 0}
	<div
		class="absolute top-full z-10 mt-1 w-full rounded-box border border-base-300 bg-base-100 p-2 shadow-lg shadow-slate-900/10"
	>
		<div class="flex items-center justify-between px-1 pb-2">
			<h2 class="text-sm font-bold text-base-content/60 uppercase tracking-wide">Recent</h2>
			<form
				method="POST"
				action="?/clearRecent"
				use:enhance={() => {
					const restore = onClear();
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
							const restore = onRemove(row.id);
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
