<script lang="ts">
	import { imageUrl } from '$lib/utils/imageUrl';
	import ImageTile from '$lib/components/ImageTile.svelte';
	import type { SearchPostItem } from '$lib/server/api/search';

	let { post }: { post: SearchPostItem } = $props();
</script>

<ImageTile postId={post.id} src={imageUrl(post.filename)} alt={post.description} />
