<script lang="ts">
	import { onMount } from 'svelte';
	import { invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { Heart, MessageCircle, UserPlus } from '@lucide/svelte';
	import { createPagination } from '$lib/createPagination.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import LoadMoreButton from '$lib/components/LoadMoreButton.svelte';
	import { fetchCursorPage } from '$lib/utils/clientFetch';
	import { relativeDate } from '$lib/utils/relativeDate';
	import type { PageData } from './$types';
	import type { Notification, NotificationType } from '$lib/types';

	let { data }: { data: PageData } = $props();

	// Optimistic overlay, also tracks which ids are already (being) marked read.
	let locallyRead = $state(new Set<string>());

	// Fires from mount and each "Load More" page, never a GET, so no passive fetch can trigger it.
	function markUnreadIds(ids: string[]) {
		if (ids.length === 0) return;
		locallyRead = new Set([...locallyRead, ...ids]);
		void fetch('/notifications', {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ ids })
		});
	}

	onMount(() => {
		invalidate('app:unreadCount');
		markUnreadIds(data.notifications.filter((n) => !n.read).map((n) => n.id));
	});

	const pagination = createPagination(
		() => ({ items: data.notifications, nextCursor: data.nextCursor }),
		async (cursor) => {
			const page = await fetchCursorPage<Notification>(fetch, '/notifications', cursor);
			invalidate('app:unreadCount');
			markUnreadIds(page.items.filter((n) => !n.read).map((n) => n.id));
			return page;
		}
	);

	const typeLabel: Record<NotificationType, string> = {
		like: 'liked your post',
		comment: 'commented on your post',
		follow: 'started following you'
	};

	const typeIcon: Record<NotificationType, typeof Heart> = {
		like: Heart,
		comment: MessageCircle,
		follow: UserPlus
	};

	const typeBadge: Record<NotificationType, string> = {
		like: 'bg-rose-500/20 text-rose-500',
		comment: 'bg-primary/20 text-primary',
		follow: 'bg-primary/20 text-primary'
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
				{@const Icon = typeIcon[notification.type]}
				{@const isRead = notification.read || locallyRead.has(notification.id)}
				<li
					class="flex items-center gap-3 rounded-2xl border border-base-300 bg-base-100 px-4 py-3 shadow-sm shadow-slate-900/5 transition-colors"
				>
					<a href={resolve(`/@${notification.actorUsername}`)} class="relative shrink-0">
						<Avatar
							username={notification.actorUsername}
							avatar={notification.actorAvatar}
							size="h-9 w-9"
						/>
						<span
							class="absolute -bottom-1 -right-1 grid h-4 w-4 place-items-center rounded-full border border-base-100 {isRead
								? 'bg-base-200 text-base-content'
								: typeBadge[notification.type]}"
							aria-hidden="true"
						>
							<Icon class="h-2.5 w-2.5" />
						</span>
					</a>
					<div class="min-w-0 flex-1">
						<p
							class="text-sm {isRead
								? 'font-normal text-base-content/60'
								: 'font-semibold text-base-content'}"
						>
							<a href={resolve(`/@${notification.actorUsername}`)} class="hover:underline"
								>{notification.actorName || notification.actorUsername}</a
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
					{#if !isRead}
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
