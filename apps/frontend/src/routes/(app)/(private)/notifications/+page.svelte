<script lang="ts">
	import { onMount } from 'svelte';
	import { invalidate } from '$app/navigation';
	import { Heart, MessageCircle, UserPlus } from '@lucide/svelte';
	import { createPagination } from '$lib/createPagination.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import LoadMoreButton from '$lib/components/LoadMoreButton.svelte';
	import { fetchJson } from '$lib/utils/clientFetch';
	import { relativeDate } from '$lib/utils/relativeDate';
	import type { PageData } from './$types';
	import type { Notification, NotificationType } from '$lib/types';

	let { data }: { data: PageData } = $props();

	onMount(() => {
		invalidate('app:unreadCount');
	});

	const pagination = createPagination(
		() => ({ items: data.notifications, nextCursor: data.nextCursor }),
		async (cursor) => {
			const res = await fetch(`/notifications?cursor=${encodeURIComponent(cursor)}`);
			const page = await fetchJson<{ items: Notification[]; nextCursor: string | null }>(res);
			invalidate('app:unreadCount');
			return page;
		}
	);

	const typeLabel: Record<NotificationType, string> = {
		like: 'liked your post',
		comment: 'commented on your post',
		follow: 'started following you'
	};
</script>

<svelte:head>
	<title>Notifications — Phasma</title>
</svelte:head>

<div class="mx-auto flex max-w-xl flex-col gap-4">
	<h1 class="text-2xl font-black text-base-content">Notifications</h1>

	{#if pagination.items.length === 0}
		<EmptyState
			icon="bell"
			title="No notifications yet"
			description="When someone likes your posts, comments, or follows you, you'll see it here."
		/>
	{:else}
		<ul class="flex flex-col gap-2" aria-label="Notifications">
			{#each pagination.items as notification (notification.id)}
				<li
					class="flex items-center gap-3 rounded-2xl border border-base-300 bg-base-100 px-4 py-3 shadow-sm shadow-slate-900/5 transition-colors {notification.read
						? 'opacity-60'
						: ''}"
				>
					<div
						class="grid h-9 w-9 shrink-0 place-items-center rounded-full {notification.read
							? 'bg-base-200 text-base-content/50'
							: 'bg-primary/10 text-primary'}"
					>
						{#if notification.type === 'like'}
							<Heart class="h-4 w-4" />
						{:else if notification.type === 'comment'}
							<MessageCircle class="h-4 w-4" />
						{:else}
							<UserPlus class="h-4 w-4" />
						{/if}
					</div>
					<div class="min-w-0 flex-1">
						<p
							class="text-sm {notification.read
								? 'font-normal text-base-content/60'
								: 'font-semibold text-base-content'}"
						>
							{typeLabel[notification.type]}
						</p>
						<time
							class="text-xs text-base-content/50"
							datetime={new Date(notification.created).toISOString()}
						>
							{relativeDate(notification.created)}
						</time>
					</div>
					{#if !notification.read}
						<span class="h-2 w-2 shrink-0 rounded-full bg-primary" aria-hidden="true"></span>
					{/if}
				</li>
			{/each}
		</ul>

		{#if !pagination.done}
			<div class="flex flex-col items-center gap-2">
				{#if pagination.error}
					<p class="text-sm text-error">{pagination.error}</p>
				{/if}
				<LoadMoreButton loading={pagination.loading} onclick={pagination.more} />
			</div>
		{/if}
	{/if}
</div>
