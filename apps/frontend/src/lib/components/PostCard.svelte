<script lang="ts">
	import { resolve } from '$app/paths';
	import { enhance } from '$app/forms';
	import { Heart, MessageCircle, Trash2, Send } from '@lucide/svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import UserLink from '$lib/components/UserLink.svelte';
	import { imageUrl } from '$lib/utils/imageUrl';
	import { relativeDate } from '$lib/utils/relativeDate';
	import { pluralize } from '$lib/utils/pluralize';
	import { fetchJson } from '$lib/utils/clientFetch';
	import type { Post, Comment } from '$lib/types';
	import Linkified from '$lib/components/Linkified.svelte';
	import Typeahead from '$lib/components/Typeahead.svelte';
	import { createTypeaheadController } from '$lib/typeaheadController.svelte';
	import { createFloatingPosition } from '$lib/utils/floatingPosition.svelte';
	import { portal } from '$lib/actions/portal';

	const mentionLinkClass =
		'mr-1.5 font-bold text-base-content transition-colors hover:text-primary';

	let {
		post: initialPost,
		currentUsername,
		singleView = false,
		comments: initialComments = [],
		nextCommentsCursor: initialNextCommentsCursor = null
	}: {
		post: Post;
		currentUsername: string | null;
		singleView?: boolean;
		comments?: Comment[];
		nextCommentsCursor?: string | null;
	} = $props();

	function initialState() {
		return {
			liked: initialPost.liked,
			likes: initialPost.likes,
			comments: initialComments,
			commentCount: initialPost.comments,
			nextCommentsCursor: initialNextCommentsCursor
		};
	}

	const initialValues = initialState();
	let liked = $state(initialValues.liked);
	let likes = $state(initialValues.likes);
	let comments = $state(initialValues.comments);
	let commentCount = $state(initialValues.commentCount);
	let nextCommentsCursor = $state(initialValues.nextCommentsCursor);
	let newCommentBody = $state('');
	let commentInput = $state<HTMLInputElement | null>(null);
	let commentTypeahead = createTypeaheadController();
	let commentDropdownPos = createFloatingPosition();
	let isSubmittingComment = $state(false);
	let isLoadingMoreComments = $state(false);
	let showDeleteModal = $state(false);
	let likeAnimating = $state(false);
	let likeForm = $state<HTMLFormElement | null>(null);
	let imageLikeBurst = $state(false);

	function playLikeAnimation() {
		likeAnimating = false;
		requestAnimationFrame(() => {
			likeAnimating = true;
			setTimeout(() => {
				likeAnimating = false;
			}, 220);
		});
	}

	function handleImageDoubleClick() {
		if (!currentUsername) return;
		imageLikeBurst = false;
		requestAnimationFrame(() => {
			imageLikeBurst = true;
			setTimeout(() => {
				imageLikeBurst = false;
			}, 700);
		});
		if (!liked) {
			likeForm?.requestSubmit();
		}
	}

	let commentLoadError = $state('');

	async function loadMoreComments() {
		if (!nextCommentsCursor || isLoadingMoreComments) return;
		isLoadingMoreComments = true;
		commentLoadError = '';
		try {
			const res = await fetch(
				`/posts/${initialPost.publicId}/comments?cursor=${encodeURIComponent(nextCommentsCursor)}`
			);
			const data = await fetchJson<{ items: Comment[]; nextCursor: string | null }>(res);
			comments = [...comments, ...data.items];
			nextCommentsCursor = data.nextCursor;
		} catch (e) {
			commentLoadError = e instanceof Error ? e.message : 'Could not load more comments.';
		} finally {
			isLoadingMoreComments = false;
		}
	}

	function handleCommentInput(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		// Read the DOM directly: bind:value's listener syncs `newCommentBody`
		// after this handler runs, so it's one keystroke stale here.
		const caret = input.selectionStart ?? input.value.length;
		commentTypeahead.handleInput(input.value, caret);
		if (commentTypeahead.token) commentDropdownPos.placeBelow(input);
	}

	function handleCommentTypeaheadSelect(value: string) {
		const next = commentTypeahead.select(newCommentBody, value, commentInput);
		if (next !== null) newCommentBody = next;
	}

	// The dropdown is portalled to <body> (see the comment input markup below)
	// and positioned in viewport coordinates, so it must be kept in sync while
	// open as the page scrolls or the viewport resizes.
	$effect(() => {
		if (commentTypeahead.items.length === 0 || !commentInput) return;
		const el = commentInput;
		const reposition = () => commentDropdownPos.placeBelow(el);
		window.addEventListener('scroll', reposition, true);
		window.addEventListener('resize', reposition);
		return () => {
			window.removeEventListener('scroll', reposition, true);
			window.removeEventListener('resize', reposition);
		};
	});
</script>

<div
	class="w-full overflow-hidden shadow-lg shadow-slate-900/5 transition-colors rounded-2xl border border-base-300 bg-base-100 text-base-content"
>
	<div
		class="relative flex items-center border-b border-base-300 bg-base-200/80 px-6 py-4 pr-16 sm:pr-20"
	>
		<UserLink
			username={initialPost.username}
			avatar={initialPost.avatar}
			primary={initialPost.username}
			secondary={initialPost.name}
		/>

		{#if initialPost.username === currentUsername}
			<button
				type="button"
				class="absolute right-5 top-1/2 inline-flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full text-base-content/50 transition-all duration-150 hover:bg-rose-500/10 hover:text-rose-600 active:scale-95 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-rose-500/50 dark:hover:text-rose-400 sm:right-6"
				title="Delete post"
				aria-label="Delete post"
				onclick={() => (showDeleteModal = true)}
			>
				<Trash2 class="h-5 w-5" />
			</button>
		{/if}
	</div>

	<div
		class="relative aspect-square w-full overflow-hidden bg-base-200"
		role="presentation"
		ondblclick={handleImageDoubleClick}
	>
		<img
			class="h-full w-full cursor-pointer object-cover transition-transform duration-700 hover:scale-[1.03]"
			src={imageUrl(initialPost.filename)}
			alt={initialPost.description ?? ''}
			loading="lazy"
			decoding="async"
			width="600"
			height="600"
		/>
		{#if imageLikeBurst}
			<div class="pointer-events-none absolute inset-0 flex items-center justify-center">
				<Heart class="h-24 w-24 fill-white text-white drop-shadow-lg animate-like-burst" />
			</div>
		{/if}
	</div>

	<div class="flex flex-col gap-4 p-6">
		<div class="flex items-center gap-5">
			{#if currentUsername}
				<form
					bind:this={likeForm}
					class="contents"
					method="POST"
					action="/posts/{initialPost.publicId}?/{liked ? 'unlike' : 'like'}"
					use:enhance={() => {
						const prevLiked = liked;
						const prevLikes = likes;
						liked = !liked;
						likes += liked ? 1 : -1;
						if (liked) playLikeAnimation();
						return async ({ result }) => {
							if (result.type === 'error' || result.type === 'failure') {
								liked = prevLiked;
								likes = prevLikes;
							}
						};
					}}
				>
					<button
						type="submit"
						class="group inline-flex items-center gap-1.5 text-sm font-semibold transition-colors active:scale-95 {liked
							? 'text-rose-500 dark:text-rose-400'
							: 'text-base-content/60'}"
						aria-label={liked ? 'Unlike post' : 'Like post'}
						aria-pressed={liked}
					>
						<Heart
							class="h-5 w-5 transition-all duration-150 ease-out {likeAnimating
								? 'animate-like-pop'
								: ''} {liked ? 'fill-rose-500 dark:fill-rose-400' : 'fill-transparent'}"
						/>
						<span class={likeAnimating ? 'animate-like-pop' : ''}>
							{likes}
							{pluralize(likes, 'like')}
						</span>
					</button>
				</form>
			{:else}
				<a
					href={resolve('/login')}
					class="group inline-flex items-center gap-1.5 text-sm font-semibold text-base-content/60 transition-colors active:scale-95"
					aria-label="Log in to like post"
				>
					<Heart class="h-5 w-5 fill-transparent" />
					<span>
						{likes}
						{pluralize(likes, 'like')}
					</span>
				</a>
			{/if}

			<a
				href={resolve(`/posts/${initialPost.publicId}`)}
				class="group inline-flex items-center gap-1.5 text-sm font-semibold text-base-content/60 transition-colors hover:text-base-content"
				aria-label="View comments"
			>
				<MessageCircle class="h-5 w-5" />
				{commentCount}
				{pluralize(commentCount, 'comment')}
			</a>
		</div>

		{#if initialPost.description && initialPost.description.length > 0}
			<div class="text-base leading-7">
				<a href={resolve(`/@${initialPost.username}`)} class={mentionLinkClass}>
					{initialPost.username}
				</a>
				<Linkified text={initialPost.description} />
			</div>
		{/if}

		<span class="text-[11px] font-bold uppercase tracking-wider text-base-content/60">
			{relativeDate(initialPost.created, 'long')}
		</span>

		{#if singleView}
			<div class="border-t border-base-300 pt-4">
				<div class="flex flex-col gap-4">
					{#if currentUsername}
						<form
							method="POST"
							action="/posts/{initialPost.publicId}?/comment"
							class="flex items-center gap-3 border-b border-base-300 pb-4"
							use:enhance={() => {
								isSubmittingComment = true;
								return async ({ result }) => {
									isSubmittingComment = false;
									if (result.type === 'success' && result.data?.comment) {
										comments = [result.data.comment as Comment, ...comments];
										commentCount += 1;
										newCommentBody = '';
										commentTypeahead.reset();
									}
								};
							}}
						>
							<div class="min-w-0 flex-1">
								<input
									bind:this={commentInput}
									type="text"
									name="body"
									bind:value={newCommentBody}
									class="w-full bg-transparent text-sm text-base-content placeholder:text-base-content/40 focus:outline-none"
									placeholder="Add a comment…"
									maxlength="400"
									autocomplete="off"
									oninput={handleCommentInput}
								/>
								{#if commentTypeahead.items.length > 0}
									<div
										use:portal
										style="position: fixed; top: {commentDropdownPos.top}px; left: {commentDropdownPos.left}px;"
									>
										<Typeahead
											onselect={handleCommentTypeaheadSelect}
											items={commentTypeahead.items}
											display={commentTypeahead.displayItem}
										/>
									</div>
								{/if}
							</div>
							<button
								type="submit"
								class="shrink-0 text-primary transition-opacity disabled:opacity-40"
								disabled={!newCommentBody.trim() || isSubmittingComment}
								aria-label="Post comment"
							>
								{#if isSubmittingComment}
									<span class="loading loading-spinner loading-xs"></span>
								{:else}
									<Send class="h-5 w-5" />
								{/if}
							</button>
						</form>
					{:else}
						<a
							href={resolve('/login')}
							class="flex items-center gap-3 border-b border-base-300 pb-4 text-sm font-semibold text-base-content/60 transition-colors hover:text-base-content"
						>
							Log in to comment
						</a>
					{/if}

					{#each comments as comment (comment.id)}
						<div class="group flex items-start gap-3">
							<a href={resolve(`/@${comment.username}`)} class="mt-0.5 block shrink-0">
								<Avatar username={comment.username} avatar={comment.avatar} size="h-8 w-8" />
							</a>
							<div class="min-w-0 flex-1 text-sm leading-6">
								<a href={resolve(`/@${comment.username}`)} class={mentionLinkClass}
									>{comment.username}</a
								>
								<Linkified text={comment.body} />
								<div class="mt-0.5 text-xs text-base-content/50">
									{relativeDate(comment.created)}
								</div>
							</div>
							{#if comment.username === currentUsername || initialPost.username === currentUsername}
								<form
									method="POST"
									action="/posts/{initialPost.publicId}?/deleteComment"
									use:enhance={() => {
										const idx = comments.findIndex((c: Comment) => c.id === comment.id);
										comments = comments.filter((c: Comment) => c.id !== comment.id);
										commentCount = Math.max(0, commentCount - 1);
										return async ({ result }) => {
											if (result.type === 'error' || result.type === 'failure') {
												const restored = [...comments];
												restored.splice(idx, 0, comment);
												comments = restored;
												commentCount += 1;
											}
										};
									}}
								>
									<input type="hidden" name="commentId" value={comment.id} />
									<button
										type="submit"
										class="-mr-2 mt-0.5 inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-base-content/50 transition-all duration-150 hover:bg-rose-500/10 hover:text-rose-600 active:scale-95 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-rose-500/50 dark:hover:text-rose-400"
										aria-label="Delete comment"
										title="Delete comment"
									>
										<Trash2 class="h-4 w-4" />
									</button>
								</form>
							{/if}
						</div>
					{/each}

					{#if commentLoadError}
						<p class="text-sm text-error">{commentLoadError}</p>
					{/if}
					{#if nextCommentsCursor}
						<button
							type="button"
							class="self-start text-sm font-semibold text-base-content/60 hover:text-base-content"
							disabled={isLoadingMoreComments}
							onclick={loadMoreComments}
						>
							{#if isLoadingMoreComments}
								<span class="loading loading-spinner loading-xs"></span>
							{:else}
								Load more comments
							{/if}
						</button>
					{/if}
				</div>
			</div>
		{/if}
	</div>
</div>

{#if showDeleteModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 backdrop-blur-[2px]"
		role="presentation"
		onclick={() => (showDeleteModal = false)}
		onkeydown={(e) => e.key === 'Escape' && (showDeleteModal = false)}
	>
		<div
			class="w-[calc(100%-2rem)] max-w-sm rounded-2xl border border-base-300 bg-base-100 p-6 text-base-content shadow-2xl"
			role="dialog"
			aria-modal="true"
			aria-labelledby="delete-modal-title"
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
		>
			<div class="grid gap-3">
				<h3 id="delete-modal-title" class="text-xl font-black">Delete post?</h3>
				<p class="text-sm leading-6 text-base-content/70">
					This will permanently remove this photo from your profile and feed.
				</p>
			</div>
			<div class="mt-6 flex justify-end gap-3">
				<button
					type="button"
					class="btn h-11 min-h-11 rounded-full px-5 font-bold"
					onclick={() => (showDeleteModal = false)}
				>
					Cancel
				</button>
				<form method="POST" action="/posts/{initialPost.publicId}?/deletePost" use:enhance>
					<button
						type="submit"
						class="btn btn-error h-11 min-h-11 rounded-full px-5 font-bold text-white"
					>
						Delete
					</button>
				</form>
			</div>
		</div>
	</div>
{/if}
