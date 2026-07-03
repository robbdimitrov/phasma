<script lang="ts">
	import PostCard from '$lib/components/PostCard.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
</script>

<svelte:head>
	<title>{data.post.description ? data.post.description.slice(0, 60) : 'Post'} — Phasma</title>
</svelte:head>

<div class="mx-auto max-w-xl">
	<PostCard
		post={data.post}
		currentUsername={data.currentUser?.username ?? null}
		singleView={true}
		comments={data.comments}
		nextCommentsCursor={data.nextCommentsCursor}
	/>
</div>
